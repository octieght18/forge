#!/usr/bin/env python3
"""Private-environment launchers; only systemd calls this installed operator code."""
import json
import os
from pathlib import Path
import sys

component, directory = sys.argv[1:]
root = Path(directory)
state = json.loads((root / "secrets.json").read_text())
env = os.environ.copy()
env["HOME"] = str(Path.home())
if component == "api":
    env.update(FORGE_PRODUCT_API="true", FORGE_HTTP_ADDR="127.0.0.1:8081",
               FORGE_DATABASE_URL=f"postgres://forge_runtime:{state['runtime']}@127.0.0.1:55436/forge?sslmode=disable",
               FORGE_OIDC_ISSUER="http://127.0.0.1:8082/realms/forge",
               FORGE_CURSOR_KEY_FILE=str(root / "cursor.key"),
               FORGE_CORPUS_POLICY_FILE=str(root / "corpus-policy.json"))
    args = [str(root / "bin/forge-api")]
elif component == "keycloak":
    env.update(KC_DB="postgres", KC_DB_URL="jdbc:postgresql://127.0.0.1:55436/keycloak",
               KC_DB_USERNAME="forge_identity", KC_DB_PASSWORD=state["identity"],
               KC_BOOTSTRAP_ADMIN_USERNAME="forge-admin", KC_BOOTSTRAP_ADMIN_PASSWORD=state["admin"],
               KC_HTTP_HOST="127.0.0.1", KC_HTTP_PORT="8082",
               KC_HTTP_MANAGEMENT_HOST="127.0.0.1", KC_HTTP_MANAGEMENT_PORT="9002",
               KC_HOSTNAME="http://127.0.0.1:8082", KC_HEALTH_ENABLED="true",
               JAVA_OPTS="-Xms256m -Xmx768m -Djava.net.preferIPv4Stack=true", KC_CACHE="local")
    args = [str(root / "keycloak-26.8.0/bin/kc.sh"), "start-dev", "--import-realm"]
else:
    raise SystemExit("Unknown native service")
os.execvpe(args[0], args, env)
