"""Deterministic Kubernetes resources; credentials are supplied separately."""
from pathlib import Path

HERE = Path(__file__).resolve().parent
NAMESPACE = "forge-local"


def obj(kind, name, spec=None, **fields):
    result = dict(apiVersion="v1", kind=kind, metadata=dict(name=name, namespace=NAMESPACE), **fields)
    if spec is not None:
        result["spec"] = spec
    return result


def secret_env(name, secret, key):
    return dict(name=name, valueFrom=dict(secretKeyRef=dict(name=secret, key=key)))


def security(uid):
    return dict(runAsNonRoot=True, runAsUser=uid, runAsGroup=uid, allowPrivilegeEscalation=False,
                capabilities=dict(drop=["ALL"]), seccompProfile=dict(type="RuntimeDefault"))


def container(name, image, uid, memory, cpu, **fields):
    return dict(name=name, image=image, imagePullPolicy="IfNotPresent", securityContext=security(uid),
                resources=dict(requests=dict(cpu="100m", memory="64Mi"), limits=dict(cpu=cpu, memory=memory)), **fields)


def deployment(name, containers, volumes=None, **fields):
    pod = dict(automountServiceAccountToken=False, terminationGracePeriodSeconds=30,
               securityContext=fields.pop("securityContext", dict(seccompProfile=dict(type="RuntimeDefault"))), containers=containers, **fields)
    if volumes:
        pod["volumes"] = volumes
    return dict(apiVersion="apps/v1", kind="Deployment", metadata=dict(name=name, namespace=NAMESPACE),
                spec=dict(replicas=1, strategy=dict(type="Recreate"), selector=dict(matchLabels=dict(app=name)),
                          template=dict(metadata=dict(labels=dict(app=name)), spec=pod)))


def service(name, port, target=None):
    return obj("Service", name, dict(type="ClusterIP", selector=dict(app=name),
                                     ports=[dict(port=port, targetPort=target or port)]))


def database(lock):
    pg = container("postgres", lock["postgres"], 999, "1Gi", "1", env=[
        secret_env("POSTGRES_PASSWORD", "postgres-admin", "password"),
        dict(name="PGDATA", value="/var/lib/postgresql/18/docker")],
        ports=[dict(containerPort=5432)],
        volumeMounts=[dict(name="data", mountPath="/var/lib/postgresql"), dict(name="socket", mountPath="/var/run/postgresql")],
        readinessProbe=dict(exec=dict(command=["pg_isready", "-U", "postgres"]), periodSeconds=2),
        startupProbe=dict(exec=dict(command=["pg_isready", "-U", "postgres"]), periodSeconds=2, failureThreshold=60))
    pv = obj("PersistentVolume", "forge-local-postgres", dict(capacity=dict(storage="20Gi"),
        accessModes=["ReadWriteOnce"], persistentVolumeReclaimPolicy="Retain", storageClassName="",
        hostPath=dict(path="/forge-data/postgres", type="Directory")))
    del pv["metadata"]["namespace"]
    pvc = obj("PersistentVolumeClaim", "postgres-data", dict(accessModes=["ReadWriteOnce"], storageClassName="",
        volumeName="forge-local-postgres", resources=dict(requests=dict(storage="20Gi"))))
    return [pv, pvc, service("postgres", 5432), deployment("postgres", [pg], [
        dict(name="data", persistentVolumeClaim=dict(claimName="postgres-data")), dict(name="socket", emptyDir={})],
        securityContext=dict(fsGroup=999, seccompProfile=dict(type="RuntimeDefault")))]


def apps(lock, api_image):
    kc = container("keycloak", lock["keycloak"], 1000, "1536Mi", "1", args=["start-dev"], env=[
        dict(name=k, value=v) for k, v in dict(KC_DB="postgres", KC_DB_URL="jdbc:postgresql://postgres:5432/keycloak",
            KC_DB_USERNAME="forge_identity", KC_HTTP_PORT="8082", KC_HTTP_MANAGEMENT_PORT="9002",
            KC_HOSTNAME="http://127.0.0.1:8082", KC_HEALTH_ENABLED="true", KC_CACHE="local",
            JAVA_OPTS_APPEND="-Xms256m -Xmx768m -Djava.net.preferIPv4Stack=true").items()
    ] + [secret_env("KC_DB_PASSWORD", "keycloak-db", "password")],
        ports=[dict(containerPort=8082), dict(containerPort=9002)],
        readinessProbe=dict(httpGet=dict(path="/health/ready", port=9002), periodSeconds=3),
        startupProbe=dict(httpGet=dict(path="/health/ready", port=9002), periodSeconds=3, failureThreshold=100))
    api = container("api", api_image, 65532, "256Mi", "1", env=[
        dict(name=k, value=v) for k, v in dict(FORGE_PRODUCT_API="true", FORGE_HTTP_ADDR="127.0.0.1:8081",
            FORGE_OIDC_ISSUER="http://127.0.0.1:8082/realms/forge", FORGE_CURSOR_KEY_FILE="/private/cursor.key",
            FORGE_CORPUS_POLICY_FILE="/policy/corpus-policy.json").items()
    ] + [secret_env("FORGE_DATABASE_URL", "api-db", "url")],
        volumeMounts=[dict(name="cursor", mountPath="/private", readOnly=True), dict(name="policy", mountPath="/policy", readOnly=True)])
    api["securityContext"]["readOnlyRootFilesystem"] = True
    proxy = container("proxy", lock["proxy"], 101, "64Mi", "500m", command=["nginx"],
        args=["-c", "/config/nginx.conf", "-g", "daemon off;"],
        volumeMounts=[dict(name="proxy-config", mountPath="/config", readOnly=True), dict(name="tmp", mountPath="/tmp")],
        ports=[dict(containerPort=8080)],
        startupProbe=dict(httpGet=dict(path="/proxy-ready", port=8080), periodSeconds=2, failureThreshold=60),
        readinessProbe=dict(httpGet=dict(path="/readyz", port=8080), periodSeconds=3),
        livenessProbe=dict(httpGet=dict(path="/healthz", port=8080), periodSeconds=10))
    proxy["securityContext"]["readOnlyRootFilesystem"] = True
    # A native sidecar becomes reachable before API OIDC discovery starts, and
    # remains alive until the API finishes draining on termination.
    proxy["restartPolicy"] = "Always"
    return [service("keycloak", 8082), deployment("keycloak", [kc]), service("api", 8081, 8080),
        obj("ConfigMap", "proxy-config", data={"nginx.conf": (HERE / "proxy.conf").read_text()}),
        deployment("api", [api], [dict(name="cursor", secret=dict(secretName="cursor-key")),
            dict(name="policy", configMap=dict(name="corpus-policy")),
            dict(name="proxy-config", configMap=dict(name="proxy-config")), dict(name="tmp", emptyDir={})], initContainers=[proxy])]


def migration(api_image, pg_image):
    worker = container("migrate", api_image, 65532, "256Mi", "1", command=["/forge-migrate"],
                       env=[secret_env("FORGE_MIGRATION_DATABASE_URL", "migration-db", "url")])
    worker["securityContext"]["readOnlyRootFilesystem"] = True
    wait = container("database-ready", pg_image, 999, "256Mi", "1", command=["sh", "-c"],
        args=['i=0; while [ "$i" -lt 30 ]; do if psql -X -q -c "SELECT 1" >/dev/null 2>&1; then exit 0; fi; i=$((i+1)); sleep 1; done; exit 1'],
        env=[dict(name=k, value=v) for k, v in dict(PGHOST="postgres", PGPORT="5432", PGDATABASE="forge", PGUSER="forge_migrator", PGCONNECT_TIMEOUT="2").items()] +
            [secret_env("PGPASSWORD", "migration-db", "password")])
    wait["securityContext"]["readOnlyRootFilesystem"] = True
    return dict(apiVersion="batch/v1", kind="Job", metadata=dict(name="forge-migrate", namespace=NAMESPACE),
        spec=dict(backoffLimit=0, activeDeadlineSeconds=120, template=dict(spec=dict(restartPolicy="Never",
            automountServiceAccountToken=False, initContainers=[wait], containers=[worker]))))


def foundation():
    return [dict(apiVersion="v1", kind="Namespace", metadata=dict(name=NAMESPACE,
        labels={"pod-security.kubernetes.io/enforce": "restricted", "pod-security.kubernetes.io/enforce-version": "v1.37"})),
        obj("ResourceQuota", "local-budget", dict(hard={"pods": "12", "requests.cpu": "3", "requests.memory": "3Gi",
            "limits.cpu": "6", "limits.memory": "6Gi", "persistentvolumeclaims": "1"}))]
