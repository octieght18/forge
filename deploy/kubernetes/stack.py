#!/usr/bin/env python3
"""Local kind stack in WSL. Root manages Docker; private state belongs to --user."""
import argparse
import base64
import fcntl
import hashlib
import json
import os
from pathlib import Path
import pwd
import re
import shutil
import socket
import subprocess
import sys
import time
import secrets
import tempfile
import urllib.request

from resources import NAMESPACE, apps, database, foundation, migration, obj
from environments import crd, infrastructure

REPO = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
LOCK = json.loads((HERE / "images.json").read_text())
CLUSTER = "forge"


class Stack:
    def __init__(self, options):
        self.options = options
        self.user = pwd.getpwnam(options.user)
        if self.user.pw_uid == 0:
            raise ValueError("Select an unprivileged state owner")
        self.root = Path(self.user.pw_dir) / ".local/share/forge-kubernetes"
        self.native = Path(self.user.pw_dir) / ".local/share/forge-native"
        if any(p.is_symlink() for p in (self.root, *self.root.parents)):
            raise ValueError("Private state path must not contain symlinks")
        self.marker = dict(format=1, uid=self.user.pw_uid, root=str(self.root), cluster=CLUSTER)
        self.env = dict(os.environ, KUBECONFIG=str(self.root / "kubeconfig"))

    def command(self, args, **kwargs):
        return subprocess.run(list(map(str, args)), check=True, env=self.env, **kwargs)

    def private(self, path, content):
        if not path.is_relative_to(self.root) or any(p.is_symlink() for p in (path, *path.parents)):
            raise ValueError("Private file path must stay inside the installation without symlinks")
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(content if isinstance(content, bytes) else content.encode())
        path.chmod(0o600)
        os.chown(path, self.user.pw_uid, self.user.pw_gid)

    def require_marker(self):
        if json.loads((self.root / "installation.json").read_text()) != self.marker:
            raise ValueError("Kubernetes installation identity mismatch")
        if self.root.stat().st_uid != self.user.pw_uid or self.root.stat().st_mode & 0o077:
            raise ValueError("Kubernetes state must be private to the selected user")

    def prepare(self):
        if self.root.exists():
            self.require_marker()
        else:
            if CLUSTER in self.command(["kind", "get", "clusters"], capture_output=True, text=True).stdout.split():
                raise ValueError("Unmanaged kind cluster named forge already exists")
            self.root.mkdir(parents=True, mode=0o700)
            os.chown(self.root, self.user.pw_uid, self.user.pw_gid)
            self.private(self.root / "installation.json", json.dumps(self.marker))
        data = self.root / "data/postgres"
        if any(p.is_symlink() for p in (self.root / "data", data, self.root / "bin", self.root / "tokens")):
            raise ValueError("Installation directories must not be symlinks")
        data.mkdir(parents=True, exist_ok=True, mode=0o700)
        os.chown(data, 999, 999)
        (self.root / "data").chmod(0o700)
        config = dict(kind="Cluster", apiVersion="kind.x-k8s.io/v1alpha4",
            networking=dict(apiServerAddress="127.0.0.1", apiServerPort=6445),
            nodes=[dict(role="control-plane", image=LOCK["node"], extraMounts=[
                dict(hostPath=str(self.root / "data"), containerPath="/forge-data")])])
        self.private(self.root / "kind.json", json.dumps(config))
        for name in ("bin", "tokens"):
            path = self.root / name
            path.mkdir(exist_ok=True, mode=0o700)
            os.chown(path, self.user.pw_uid, self.user.pw_gid)
        helper = self.root / "bin/forge-login"
        if not helper.exists():
            self.private(helper, (self.native / "bin/forge-login").read_bytes())
            helper.chmod(0o700)

    def kube(self, *args, **kwargs):
        return self.command(["kubectl", "--context=kind-forge", "-n", NAMESPACE, *args], **kwargs)

    def apply(self, items):
        payload = json.dumps(dict(apiVersion="v1", kind="List", items=items)).encode()
        result = self.kube("apply", "-f", "-", input=payload, capture_output=True)
        # Never echo secret manifests or private SQL in diagnostics.
        if result.returncode:
            raise RuntimeError("Resource apply failed; inspect local Kubernetes events")

    def secret(self, name, fields):
        return obj("Secret", name, type="Opaque", data={k: base64.b64encode(v.encode()).decode() for k, v in fields.items()})

    def cluster(self):
        names = self.command(["kind", "get", "clusters"], capture_output=True, text=True).stdout.split()
        if CLUSTER not in names:
            self.command(["kind", "create", "cluster", "--name", CLUSTER, "--config", self.root / "kind.json", "--wait", "180s"])
            self.root.joinpath("kubeconfig").chmod(0o600)
            os.chown(self.root / "kubeconfig", self.user.pw_uid, self.user.pw_gid)
        else:
            self.command(["docker", "start", "forge-control-plane"], stdout=subprocess.DEVNULL)
        self.wait(lambda: self.kube("get", "nodes", capture_output=True).returncode == 0, 180)
        self.kube("wait", "--for=condition=Ready", "node/forge-control-plane", "--timeout=180s")
        self.kube("rollout", "status", "-n", "kube-system", "deployment/coredns", "--timeout=180s")
        self.apply(foundation())
        for name in ("postgres", "keycloak", "proxy"):
            self.load_image(LOCK[name])

    def load_image(self, image):
        # Docker's containerd store retains a multi-platform index even when only
        # amd64 layers exist. Export a complete single-platform archive for kind.
        if subprocess.run(["docker", "image", "inspect", image], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode:
            self.command(["docker", "pull", "--platform=linux/amd64", image], stdout=subprocess.DEVNULL)
        tag = image.split("@")[0]
        self.command(["docker", "tag", image, tag])
        archive = self.root / "image.tar"
        try:
            self.command(["docker", "save", "--platform=linux/amd64", "-o", archive, tag])
            archive.chmod(0o600)
            self.command(["kind", "load", "image-archive", archive, "--name", CLUSTER])
        finally:
            archive.unlink(missing_ok=True)

    def wait(self, predicate, seconds=120):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            try:
                if predicate():
                    return
            except (OSError, subprocess.CalledProcessError):
                pass
            time.sleep(1)
        raise RuntimeError("Readiness timed out; inspect local pod events/logs")

    def pod(self, app):
        data = json.loads(self.kube("get", "pods", "-l", "app=" + app, "-o", "json", capture_output=True).stdout)
        return next(item["metadata"]["name"] for item in data["items"] if not item["metadata"].get("deletionTimestamp"))

    def sql(self, text, db="postgres"):
        result = self.kube("exec", "-i", self.pod("postgres"), "--", "sh", "-c",
            'PGPASSWORD="$POSTGRES_PASSWORD" exec psql -X -q -A -t -v ON_ERROR_STOP=1 -U postgres -d ' + db,
            input=text.encode(), capture_output=True)
        return result.stdout.decode()

    def snapshot(self, db, native=False):
        # Hash every row, sorted by its JSON representation; table data never leaves private state.
        script = "SET TIME ZONE 'UTC'; SELECT format('SELECT json_build_object(''table'',%L,''rows'',count(*),''digest'',md5(coalesce(string_agg(row_to_json(t)::text,'''' ORDER BY row_to_json(t)::text COLLATE \"C\"),''''))) FROM %I.%I t;', table_schema||'.'||table_name,table_schema,table_name) FROM information_schema.tables WHERE table_type='BASE TABLE' AND table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_schema,table_name\\gexec\n"
        if not native:
            return [json.loads(line) for line in self.sql(script, db).splitlines() if line.strip()]
        values = json.loads((self.root / "secrets.json").read_text())
        env = dict(os.environ, PGPASSWORD=values["postgres"])
        result = subprocess.run(["/usr/lib/postgresql/18/bin/psql", "-X", "-q", "-A", "-t", "-v", "ON_ERROR_STOP=1",
            "-h", "127.0.0.1", "-p", "55436", "-U", "postgres", "-d", db], input=script, capture_output=True, text=True, env=env, check=True)
        return [json.loads(line) for line in result.stdout.splitlines() if line.strip()]

    def import_native(self):
        self.prepare()
        if (self.root / "imported.json").exists() or (self.root / "data/postgres/18/docker/PG_VERSION").exists():
            raise ValueError("Import is one-time into an empty Kubernetes database; existing data retained")
        sys.path.insert(0, str(REPO / "deploy/native"))
        # Import the native manager under a distinct name to avoid this module's name.
        import importlib.util
        spec = importlib.util.spec_from_file_location("native_stack", REPO / "deploy/native/stack.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        native = module.Stack(argparse.Namespace(user=self.options.user, state_dir=None, go="go", keycloak_archive=None, confirm=None))
        native.require_marker()
        native.stop()  # Quiesce API and identity writes before both snapshots.
        native.start("postgres", [module.PG / "postgres", "-D", self.native / "postgres", "-h", "127.0.0.1", "-p", "55436", "-k", self.native / "pg-socket"], "1G", "SIGINT")
        native.wait("PostgreSQL", lambda: subprocess.run([str(module.PG / "pg_isready"), "-h", "127.0.0.1", "-p", "55436"], stdout=subprocess.DEVNULL).returncode == 0)
        for name in ("secrets.json", "cursor.key", "corpus-policy.json", "validation-proof.json"):
            if (self.native / name).exists():
                self.private(self.root / name, (self.native / name).read_bytes())
        values = self.values()
        locale = subprocess.run([str(module.PG / "psql"), "-X", "-A", "-t", "-h", "127.0.0.1", "-p", "55436", "-U", "postgres", "-d", "postgres", "-c",
            "SELECT datcollate,datctype,datlocprovider,pg_encoding_to_char(encoding) FROM pg_database WHERE datname IN ('forge','keycloak') ORDER BY datname"],
            env=dict(os.environ, PGPASSWORD=values["postgres"]), capture_output=True, text=True, check=True)
        if locale.stdout.splitlines() != ["C.UTF-8|C.UTF-8|c|UTF8"] * 2:
            raise ValueError("Source database locale differs from the supported native installation; review before copying")
        backup = self.root / "backup"
        if backup.is_symlink():
            raise ValueError("Backup directory must not be a symlink")
        backup.mkdir(exist_ok=True, mode=0o700)
        before = {}
        for db in ("forge", "keycloak"):
            before[db] = self.snapshot(db, native=True)
            dump = backup / (db + ".dump")
            if dump.is_symlink():
                raise ValueError("Backup file must not be a symlink")
            with dump.open("wb") as output:
                dump.chmod(0o600)
                subprocess.run(["/usr/lib/postgresql/18/bin/pg_dump", "-h", "127.0.0.1", "-p", "55436", "-U", "postgres", "-Fc", db],
                    env=dict(os.environ, PGPASSWORD=values["postgres"]), stdout=output, stderr=subprocess.PIPE, check=True)
        self.private(backup / "before.json", json.dumps(before))
        native.stop()
        self.cluster()
        self.apply([self.secret("postgres-admin", dict(password=values["postgres"]))] + database(LOCK))
        self.kube("rollout", "status", "deployment/postgres", "--timeout=180s")
        roles = ""
        for role, key in (("forge_migrator", "migrator"), ("forge_runtime", "runtime"), ("forge_dispatcher", "dispatcher"), ("forge_identity", "identity")):
            roles += f"CREATE ROLE {role} LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD '{values[key]}';\n"
        roles += "CREATE DATABASE forge OWNER forge_migrator TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8'; CREATE DATABASE keycloak OWNER forge_identity TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8';\nREVOKE ALL ON DATABASE forge,keycloak FROM PUBLIC; GRANT CONNECT ON DATABASE forge TO forge_migrator,forge_runtime,forge_dispatcher; GRANT CONNECT ON DATABASE keycloak TO forge_identity;\n"
        self.sql(roles)
        for db in ("forge", "keycloak"):
            dump = backup / (db + ".dump")
            if dump.stat().st_size > 256 * 1024 * 1024:
                raise ValueError("Local demo restore limit is 256 MiB per dump; use an operator-reviewed large-data restore")
            self.restore_dump(db, dump)
        after = {db: self.snapshot(db) for db in before}
        if before != after:
            raise RuntimeError("Restored table data differs; private backups/native originals retained")
        self.private(self.root / "imported.json", json.dumps(dict(all_table_data_equal=True, databases=list(before), native_originals_retained=True)))
        print("Both databases copied: all table counts and row digests match; native originals retained")
        self.up()

    def restore_dump(self, db, dump):
        # pg_restore may exit before consuming trailing custom-archive bytes,
        # leaving kubectl's streaming stdin blocked. First consume the complete
        # input into a private Pod temporary file, then restore without stdin.
        self.kube("exec", "-i", self.pod("postgres"), "--", "sh", "-c",
            "umask 077; cat > /tmp/forge-restore.dump", input=dump.read_bytes(), capture_output=True, timeout=180)
        self.kube("exec", self.pod("postgres"), "--", "sh", "-c",
            'trap \'rm -f /tmp/forge-restore.dump\' EXIT; PGPASSWORD="$POSTGRES_PASSWORD" pg_restore --exit-on-error -U postgres -d ' + db + ' /tmp/forge-restore.dump',
            capture_output=True, timeout=180)

    def values(self):
        values = json.loads((self.root / "secrets.json").read_text())
        expected = {"postgres", "migrator", "runtime", "dispatcher", "identity", "admin", "ahmad", "second-owner", "operator"}
        if set(values) not in (expected, expected | {"environment"}) or any(not isinstance(v,str) or not re.fullmatch("[0-9a-f]{48}", v) for v in values.values()):
            raise ValueError("Malformed private credentials; values withheld")
        return values

    def environment_credentials(self):
        values = self.values()
        if "environment" not in values:
            values["environment"] = secrets.token_hex(24)
            path = self.root / "secrets.json"
            if path.is_symlink():
                raise ValueError("Credential file must not be a symlink")
            temporary = None
            try:
                with tempfile.NamedTemporaryFile(mode="w", dir=self.root, prefix=".credentials-", delete=False) as output:
                    temporary = Path(output.name)
                    os.fchmod(output.fileno(),0o600)
                    os.fchown(output.fileno(),self.user.pw_uid,self.user.pw_gid)
                    json.dump(values,output)
                    output.flush()
                    os.fsync(output.fileno())
                os.replace(temporary,path)
            finally:
                if temporary is not None:
                    temporary.unlink(missing_ok=True)
        return values

    def build(self):
        source = ["go.mod", "go.sum"] + [str(p.relative_to(REPO)) for folder in ("cmd", "internal") for p in (REPO / folder).rglob("*") if p.is_file()]
        digest = hashlib.sha256()
        for name in sorted(source):
            digest.update(name.encode())
            digest.update((REPO / name).read_bytes())
        digest.update((REPO / "Dockerfile").read_bytes())
        digest.update(LOCK["builder"].encode())
        image = "forge-api:local-" + digest.hexdigest()[:16]
        self.command(["docker", "build", "--build-arg", "GO_IMAGE=" + LOCK["builder"], "-t", image, "."], cwd=REPO)
        archive = self.root / "image.tar"
        try:
            self.command(["docker", "save", "--platform=linux/amd64", "-o", archive, image])
            archive.chmod(0o600)
            self.command(["kind", "load", "image-archive", archive, "--name", CLUSTER])
        finally:
            archive.unlink(missing_ok=True)
        return image

    def up(self):
        self.require_marker()
        if not (self.root / "imported.json").exists():
            raise ValueError("Complete one-time import-native before startup")
        self.check_ports()
        self.cluster()
        values = self.environment_credentials()
        self.apply([self.secret("postgres-admin", dict(password=values["postgres"]))] + database(LOCK))
        self.kube("rollout", "status", "deployment/postgres", "--timeout=180s")
        url = lambda role, key: f"postgres://{role}:{values[key]}@postgres:5432/forge?sslmode=disable"
        self.apply([self.secret("api-db", dict(url=url("forge_runtime", "runtime"))),
            self.secret("migration-db", dict(url=url("forge_migrator", "migrator"), password=values["migrator"])),
            self.secret("keycloak-db", dict(password=values["identity"])),
            self.secret("cursor-key", {"cursor.key": (self.root / "cursor.key").read_text()}),
            obj("ConfigMap", "corpus-policy", data={"corpus-policy.json": (self.root / "corpus-policy.json").read_text()})])
        image = self.build()
        # Explicit migration job; API never runs migrations or receives its role.
        self.kube("delete", "job", "forge-migrate", "--ignore-not-found", "--wait=true", stdout=subprocess.DEVNULL)
        self.apply([migration(image, LOCK["postgres"])])
        self.kube("wait", "--for=condition=complete", "job/forge-migrate", "--timeout=120s")
        for name in ("runtime-grants.sql", "dispatcher-grants.sql"):
            self.sql((REPO / "deploy/postgres" / name).read_text(), "forge")
        self.sql("SELECT 'CREATE ROLE forge_environment LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname='forge_environment')\\gexec\n"
                 f"ALTER ROLE forge_environment PASSWORD '{values['environment']}';\n")
        self.sql((REPO / "deploy/postgres/environment-grants.sql").read_text(), "forge")
        self.apply([self.secret("environment-db",dict(url=url("forge_environment","environment"))), crd()])
        self.kube("wait","--for=condition=Established","crd/forgeenvironments.platform.forge.local","--timeout=60s")
        self.apply(infrastructure(image))
        self.kube("rollout","status","deployment/environment-controller","--timeout=180s")
        resources = apps(LOCK, image)
        self.apply([item for item in resources if item["metadata"]["name"] != "api"])
        self.kube("rollout", "status", "deployment/keycloak", "--timeout=300s")
        self.apply([item for item in resources if item["metadata"]["name"] == "api"])
        self.kube("rollout", "status", "deployment/api", "--timeout=180s")
        self.forwards()
        self.private(self.root / "image.json", json.dumps(dict(image=image)))
        print("Kubernetes Forge ready: API http://127.0.0.1:8081; identity http://127.0.0.1:8082")

    def forwards(self):
        self.check_ports()
        for name, port in (("api", 8081), ("keycloak", 8082)):
            unit = "forge-kubernetes-forward-" + name
            if subprocess.run(["systemctl", "is-active", "--quiet", unit]).returncode == 0:
                continue
            with socket.socket() as sock:
                try:
                    sock.bind(("127.0.0.1", port))
                except OSError:
                    raise ValueError(f"Loopback port {port} occupied; stop native stack or conflicting application")
            self.command(["systemd-run", "--quiet", "--collect", "--unit=" + unit, "--uid=" + self.user.pw_name,
                "--property=UMask=0077", "--property=Restart=on-failure", "--property=RestartSec=2",
                "--setenv=KUBECONFIG=" + str(self.root / "kubeconfig"),
                "/usr/local/bin/kubectl", "--context=kind-forge", "-n", NAMESPACE, "port-forward", "--address=127.0.0.1",
                "service/" + name, f"{port}:{port}"])
        self.wait(lambda: urllib.request.urlopen("http://127.0.0.1:8081/readyz", timeout=2).status == 200)

    def check_ports(self):
        for name, port in (("api", 8081), ("keycloak", 8082)):
            if subprocess.run(["systemctl", "is-active", "--quiet", "forge-kubernetes-forward-" + name]).returncode == 0:
                continue
            with socket.socket() as sock:
                try:
                    sock.bind(("127.0.0.1", port))
                except OSError:
                    raise ValueError(f"Loopback port {port} occupied; stop the conflicting stack before startup")

    def stop(self):
        self.require_marker()
        for name in ("api", "keycloak"):
            subprocess.run(["systemctl", "stop", "forge-kubernetes-forward-" + name], check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        running = self.command(["docker", "inspect", "--format={{.State.Running}}", "forge-control-plane"], capture_output=True, text=True).stdout.strip()
        if running == "true":
            targets=["api","keycloak"]
            if self.kube("get","deployment/environment-controller","--ignore-not-found","-o","name",capture_output=True).stdout.strip():
                targets.append("environment-controller")
            self.kube("scale", *["deployment/"+name for name in targets], "--replicas=0")
            self.kube("wait", "--for=delete", "pod", "-l", "app=api", "--timeout=60s")
            self.kube("wait", "--for=delete", "pod", "-l", "app=keycloak", "--timeout=60s")
            if "environment-controller" in targets:
                self.kube("wait", "--for=delete", "pod", "-l", "app=environment-controller", "--timeout=60s")
            self.kube("scale", "deployment/postgres", "--replicas=0")
            self.kube("wait", "--for=delete", "pod", "-l", "app=postgres", "--timeout=60s")
            self.command(["docker", "stop", "forge-control-plane"], stdout=subprocess.DEVNULL)
        print("Kubernetes stack stopped; cluster, databases, backups and native originals retained")

    def status(self):
        self.require_marker()
        running = self.command(["docker", "inspect", "--format={{.State.Running}}", "forge-control-plane"], capture_output=True, text=True).stdout.strip()
        if running != "true":
            print("Kubernetes stack stopped; data and cluster retained")
            return
        self.kube("get", "deployments,pods,pvc")

    def hold(self):
        self.require_marker()
        with (self.root / "hold.lock").open("a") as lock:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                return
            while any(subprocess.run(["systemctl", "is-active", "--quiet", "forge-kubernetes-forward-" + name]).returncode == 0 for name in ("api", "keycloak")):
                time.sleep(2)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("action", choices=("import-native", "up", "stop", "status", "hold"))
    p.add_argument("--user", required=True)
    args = p.parse_args()
    if os.geteuid() != 0 and args.action != "hold":
        p.error("Use the Windows WSL wrapper or sudo; Docker access is privileged")
    instance = Stack(args)
    if args.action == "hold":
        instance.hold()
        return
    if args.action in ("up", "import-native"):
        instance.prepare()
    else:
        instance.require_marker()
    with (instance.root / "operation.lock").open("a") as lock:
        os.chmod(instance.root / "operation.lock", 0o600)
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise ValueError("Another Kubernetes lifecycle operation is in progress")
        getattr(instance, args.action.replace("-", "_"))()


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, subprocess.CalledProcessError, subprocess.TimeoutExpired) as error:
        print("Kubernetes operation failed: " + (str(error) if isinstance(error, (ValueError, RuntimeError)) else "local command failed; private diagnostics withheld"), file=sys.stderr)
        sys.exit(1)
