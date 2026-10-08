#!/usr/bin/env python3
"""Native local stack. Run through sudo/WSL root; services run as the selected user."""
import argparse
import base64
import fcntl
import hashlib
import json
import os
from pathlib import Path
import pwd
import re
import secrets
import shutil
import socket
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request

REPO = Path(__file__).resolve().parents[2]
VERSION = "26.8.0"
CHECKSUM = "9e41da899f838a58cd510fc98ed4f7cadc715aed5683e42aca20a0c9a2a3980a"
PG = Path("/usr/lib/postgresql/18/bin")


class Stack:
    def __init__(self, options):
        self.options = options
        self.user = pwd.getpwnam(options.user)
        if self.user.pw_uid == 0:
            raise ValueError("Select an unprivileged service user")
        self.home = Path(self.user.pw_dir).resolve()
        raw = Path(options.state_dir or self.home / ".local/share/forge-native")
        if not raw.is_absolute() or any(p.is_symlink() for p in (raw, *raw.parents)):
            raise ValueError("State directory must be absolute and have no symlink components")
        self.root = raw.resolve()
        if not self.root.is_relative_to(self.home) or self.root == self.home or len(self.root.relative_to(self.home).parts) < 2:
            raise ValueError("State must be a dedicated directory below the service user's home")
        self.prefix = "forge-native-" + hashlib.sha256(str(self.root).encode()).hexdigest()[:10]
        self.marker = {"format": 1, "uid": self.user.pw_uid, "root": str(self.root), "units": self.prefix}
        self.env = dict(os.environ, HOME=str(self.home), GOTOOLCHAIN="local")

    def command(self, args, user=False, **kwargs):
        if user:
            args = ["runuser", "-u", self.user.pw_name, "--", *map(str, args)]
        return subprocess.run(list(map(str, args)), check=True, env=self.env, **kwargs)

    def write(self, path, content):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        path.chmod(0o600)
        os.chown(path, self.user.pw_uid, self.user.pw_gid)

    def require_marker(self):
        if json.loads((self.root / "installation.json").read_text()) != self.marker:
            raise ValueError("Installation identity does not match; refusing lifecycle operation")

    def prepare(self):
        if self.root.exists():
            self.require_marker()
        else:
            self.root.mkdir(parents=True, mode=0o700)
            os.chown(self.root, self.user.pw_uid, self.user.pw_gid)
            self.write(self.root / "installation.json", json.dumps(self.marker))
        if self.root.stat().st_uid != self.user.pw_uid or self.root.stat().st_mode & 0o077:
            raise ValueError("State directory must be private to the selected service user")
        for name in ("bin", "pg-socket", "tokens"):
            directory = self.root / name
            directory.mkdir(exist_ok=True, mode=0o700)
            os.chown(directory, self.user.pw_uid, self.user.pw_gid)
        self.prepare_secrets()
        if not (self.root / "cursor.key").exists():
            self.write(self.root / "cursor.key", base64.b64encode(secrets.token_bytes(32)).decode())
        if not (self.root / "corpus-policy.json").exists():
            self.write(self.root / "corpus-policy.json", (REPO / "deploy/corpus-policy.example.json").read_text())
        if not (self.root / "postgres/PG_VERSION").exists():
            self.write(self.root / "postgres-password", self.values["postgres"])
            self.command([PG / "initdb", "-D", self.root / "postgres", "-U", "postgres", "--auth-local=trust", "--auth-host=scram-sha-256", "--pwfile", self.root / "postgres-password", "--encoding=UTF8", "--locale=C.UTF-8"], user=True, stdout=subprocess.DEVNULL)
        self.write(self.root / "runtime.py", (REPO / "deploy/native/runtime.py").read_text())

    def prepare_secrets(self):
        secret_file = self.root / "secrets.json"
        legacy = {"postgres", "migrator", "runtime", "identity", "admin", "ahmad", "second-owner", "operator"}
        names = legacy | {"dispatcher"}
        if not secret_file.exists():
            values = {name: secrets.token_hex(24) for name in sorted(names)}
            self.write_secrets(secret_file, values)
        self.values = json.loads(secret_file.read_text())
        if not isinstance(self.values, dict) or set(self.values) not in (legacy, names) or any(not isinstance(value, str) or not re.fullmatch(r"[0-9a-f]{48}", value) for value in self.values.values()):
            raise ValueError("Generated secret configuration is malformed; values withheld")
        if "dispatcher" not in self.values:
            self.values["dispatcher"] = secrets.token_hex(24)
            self.write_secrets(secret_file, self.values)

    def write_secrets(self, path, values):
        # An interrupted retained-state upgrade must not truncate existing passwords.
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, prefix=".secrets-", delete=False) as output:
                temporary = Path(output.name)
                os.fchmod(output.fileno(), 0o600)
                os.fchown(output.fileno(), self.user.pw_uid, self.user.pw_gid)
                output.write(json.dumps(values, indent=2))
                output.flush()
                os.fsync(output.fileno())
            os.replace(temporary, path)
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)

    def active(self, component):
        return subprocess.run(["systemctl", "is-active", "--quiet", self.prefix + "-" + component], check=False).returncode == 0

    def start(self, component, args, memory, signal="SIGTERM"):
        if self.active(component):
            return
        self.command(["systemd-run", "--quiet", "--collect", "--unit=" + self.prefix + "-" + component,
                      "--uid=" + self.user.pw_name, "--gid=" + str(self.user.pw_gid),
                      "--property=MemoryMax=" + memory, "--property=CPUQuota=100%",
                      "--property=UMask=0077", "--property=NoNewPrivileges=yes",
                      "--property=TimeoutStopSec=30", "--property=KillSignal=" + signal,
                      "--property=Restart=no", *map(str, args)])

    def wait(self, component, check, seconds=120):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            try:
                if check():
                    return
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(1)
        raise RuntimeError(component + " readiness failed; inspect its systemd journal locally")

    def sql(self, sql, database="postgres", migrator=False):
        env = dict(self.env, PGPASSWORD=self.values["migrator"] if migrator else self.values["postgres"])
        args = [PG / "psql", "-X", "-q", "-v", "ON_ERROR_STOP=1", "-h", "127.0.0.1", "-p", "55436", "-U", "forge_migrator" if migrator else "postgres", "-d", database]
        result = subprocess.run(list(map(str, args)), input=sql, text=True, env=env, capture_output=True)
        if result.returncode:
            raise RuntimeError("Database bootstrap/grants failed; private SQL diagnostics withheld")

    def install_keycloak(self):
        target = self.root / ("keycloak-" + VERSION)
        if not target.exists():
            archive = Path(self.options.keycloak_archive) if self.options.keycloak_archive else self.root / "keycloak.tar.gz"
            if not archive.exists():
                urllib.request.urlretrieve(f"https://github.com/keycloak/keycloak/releases/download/{VERSION}/keycloak-{VERSION}.tar.gz", archive)
            with archive.open("rb") as stream:
                digest = hashlib.file_digest(stream, "sha256").hexdigest()
            if digest != CHECKSUM:
                raise RuntimeError("Keycloak archive checksum mismatch")
            with tarfile.open(archive) as tar:
                tar.extractall(self.root, filter="data")
            for base, dirs, files in os.walk(target):
                os.chown(base, self.user.pw_uid, self.user.pw_gid)
                for name in files:
                    os.chown(Path(base) / name, self.user.pw_uid, self.user.pw_gid)
        realm_path = target / "data/import/forge-realm.json"
        if not realm_path.exists():
            realm = json.loads((REPO / "deploy/native/realm.json").read_text())
            realm["users"] = [{"username": name, "firstName": "Ahmad" if name == "ahmad" else name,
                               "lastName": "Demo", "email": name + "@example.invalid", "enabled": True, "emailVerified": True,
                               "credentials": [{"type": "password", "value": self.values[name], "temporary": False}],
                               "clientRoles": {"forge-api": ["operator" if name == "operator" else "developer"]}}
                              for name in ("ahmad", "second-owner", "operator")]
            realm_path.parent.mkdir(parents=True, exist_ok=True)
            os.chown(realm_path.parent, self.user.pw_uid, self.user.pw_gid)
            self.write(realm_path, json.dumps(realm))

    def up(self):
        for port in (55436, 8081, 8082, 9002):
            component = "postgres" if port == 55436 else "api" if port == 8081 else "keycloak"
            if not self.active(component):
                with socket.socket() as probe:
                    probe.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                    probe.bind(("127.0.0.1", port))
        self.prepare()
        toolchain = self.root / "go-toolchain.json"
        if self.options.go == "go" and not shutil.which("go") and toolchain.exists():
            self.options.go = json.loads(toolchain.read_text())["go"]
        self.write(toolchain, json.dumps({"go": self.options.go}))
        self.command(["java", "-version"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.install_keycloak()
        # Build with the pinned go.mod toolchain; API always starts after explicit migrations.
        if self.active("api"):
            self.command(["systemctl", "stop", self.prefix + "-api"])
        for name, package in (("forge-api", "api"), ("forge-migrate", "forge-migrate"), ("forge-login", "forge-login")):
            self.command([self.options.go, "build", "-o", self.root / "bin" / name, "./cmd/" + package], user=True, cwd=REPO)
        self.start("postgres", [PG / "postgres", "-D", self.root / "postgres", "-h", "127.0.0.1", "-p", "55436", "-k", self.root / "pg-socket"], "1G", "SIGINT")
        self.wait("PostgreSQL", lambda: subprocess.run([str(PG / "pg_isready"), "-h", "127.0.0.1", "-p", "55436"], stdout=subprocess.DEVNULL).returncode == 0)
        if not (self.root / "database-bootstrap.done").exists():
            # Generated secrets are hexadecimal, so SQL interpolation has no quoting ambiguity.
            sql = ""
            for role, key in (("forge_migrator", "migrator"), ("forge_runtime", "runtime"), ("forge_identity", "identity")):
                sql += f"SELECT 'CREATE ROLE {role} LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname='{role}')\\gexec\nALTER ROLE {role} PASSWORD '{self.values[key]}';\n"
            for db, owner in (("forge", "forge_migrator"), ("keycloak", "forge_identity")):
                sql += f"SELECT 'CREATE DATABASE {db} OWNER {owner} ENCODING ''UTF8''' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname='{db}')\\gexec\nREVOKE ALL ON DATABASE {db} FROM PUBLIC;\n"
            sql += "GRANT CONNECT ON DATABASE forge TO forge_migrator, forge_runtime;\nGRANT CONNECT ON DATABASE keycloak TO forge_identity;\n"
            self.sql(sql)
            self.write(self.root / "database-bootstrap.done", "1")
        # Add the dispatcher role even on retained installations whose original
        # bootstrap marker predates F12. No dispatcher process is launched.
        self.sql("SELECT 'CREATE ROLE forge_dispatcher LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname='forge_dispatcher')\\gexec\n"
                 f"ALTER ROLE forge_dispatcher PASSWORD '{self.values['dispatcher']}';\nGRANT CONNECT ON DATABASE forge TO forge_dispatcher;\n")
        migration_env = dict(self.env, FORGE_MIGRATION_DATABASE_URL=f"postgres://forge_migrator:{self.values['migrator']}@127.0.0.1:55436/forge?sslmode=disable")
        subprocess.run(["runuser", "-u", self.user.pw_name, "--", str(self.root / "bin/forge-migrate")], check=True, env=migration_env)
        self.sql((REPO / "deploy/postgres/runtime-grants.sql").read_text(), "forge", migrator=True)
        self.sql((REPO / "deploy/postgres/dispatcher-grants.sql").read_text(), "forge", migrator=True)
        self.start("keycloak", ["/usr/bin/python3", self.root / "runtime.py", "keycloak", self.root], "1536M")
        self.wait("Keycloak", lambda: urllib.request.urlopen("http://127.0.0.1:9002/health/ready", timeout=2).status == 200, 180)
        self.wait("Keycloak realm", lambda: urllib.request.urlopen("http://127.0.0.1:8082/realms/forge/.well-known/openid-configuration", timeout=2).status == 200)
        self.start("api", ["/usr/bin/python3", self.root / "runtime.py", "api", self.root], "256M")
        self.wait("Forge API", lambda: urllib.request.urlopen("http://127.0.0.1:8081/readyz", timeout=3).status == 200)
        print("Native Forge ready: API http://127.0.0.1:8081; identity http://127.0.0.1:8082")

    def stop(self):
        self.require_marker()
        for component in ("api", "keycloak", "postgres"):
            if self.active(component):
                self.command(["systemctl", "stop", self.prefix + "-" + component])
        print("Native services stopped; database, identities and private configuration retained")

    def status(self):
        self.require_marker()
        print(json.dumps({"state_dir": str(self.root), "units": {name: {"unit": self.prefix + "-" + name, "active": self.active(name)} for name in ("api", "keycloak", "postgres")}}, indent=2))

    def hold(self):
        # systemd services alone do not keep WSL alive. One host-launched WSL
        # process holds the instance while this installation has active units.
        self.require_marker()
        with (self.root / "wsl-hold.lock").open("a") as lock:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                return
            inactive = 0
            while inactive < 5:
                inactive = 0 if any(self.active(name) for name in ("api", "keycloak", "postgres")) else inactive + 1
                time.sleep(1)

    def purge(self):
        self.require_marker()
        if self.options.confirm != str(self.root):
            raise ValueError("Deletion requires --confirm with the exact absolute state directory; back up first")
        if any(self.active(name) for name in ("api", "keycloak", "postgres")):
            raise ValueError("Stop this installation before explicitly deleting its retained data")
        # __init__ checked the canonical dedicated user-home target and every
        # ancestor for symlinks; the marker binds it to this installation.
        shutil.rmtree(self.root)
        print("Explicitly deleted this installation's retained native data")


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("action", choices=("up", "stop", "status", "hold", "purge"))
    p.add_argument("--user", required=True)
    p.add_argument("--state-dir")
    p.add_argument("--go", default="go")
    p.add_argument("--keycloak-archive")
    p.add_argument("--confirm")
    options = p.parse_args()
    if os.geteuid() != 0 and options.action != "hold":
        p.error("Use sudo or the Windows WSL entry point; services run without root")
    getattr(Stack(options), options.action)()


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, subprocess.CalledProcessError) as error:
        detail = str(error) if isinstance(error, (ValueError, RuntimeError)) else ("OS error " + str(error.errno) if isinstance(error, OSError) else "command failed; check prerequisites and private systemd diagnostics")
        print("Native stack operation failed: " + detail, file=sys.stderr)
        sys.exit(1)
