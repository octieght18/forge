"""Operator-only environment API and controller infrastructure."""
from resources import obj, deployment, container, NAMESPACE, secret_env

GROUP = "platform.forge.local"
UUID = r"^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"


def crd():
    identity = dict(type="object", required=["issuer", "subject"], properties={
        "issuer": dict(type="string", minLength=1, maxLength=2048),
        "subject": dict(type="string", minLength=1, maxLength=255)})
    spec = dict(type="object", required=["workloadID", "owner", "profile"], properties={
        "workloadID": dict(type="string", pattern=UUID, maxLength=36), "owner": identity,
        "profile": dict(type="string", enum=["small-v1"])})
    spec["x-kubernetes-validations"] = [dict(rule="self == oldSelf", message="Environment identity/profile is immutable")]
    condition = dict(type="object", required=["type", "status", "reason", "message", "lastTransitionTime", "observedGeneration"], properties={
        "type": dict(type="string"), "status": dict(type="string", enum=["True", "False", "Unknown"]),
        "reason": dict(type="string"), "message": dict(type="string"),
        "lastTransitionTime": dict(type="string", format="date-time"), "observedGeneration":dict(type="integer", format="int64")})
    schema = dict(type="object", required=["spec"], properties={
        "spec": spec, "status": dict(type="object", properties={
            "phase": dict(type="string", enum=["Pending", "Provisioning", "Ready", "Failed", "Deleting"]),
            "observedGeneration":dict(type="integer", format="int64"),
            "namespace": dict(type="object", required=["name", "uid"], properties={"name":dict(type="string"), "uid":dict(type="string")}),
            "conditions": dict(type="array", items=condition, **{"x-kubernetes-list-type":"map", "x-kubernetes-list-map-keys":["type"]})})},
        **{"x-kubernetes-validations":[dict(rule="self.metadata.name == 'workload-' + self.spec.workloadID", message="Name must be workload- plus the workload UUID")]})
    return dict(apiVersion="apiextensions.k8s.io/v1", kind="CustomResourceDefinition", metadata=dict(name="forgeenvironments."+GROUP),
        spec=dict(group=GROUP, scope="Cluster", names=dict(kind="ForgeEnvironment", plural="forgeenvironments", singular="forgeenvironment", shortNames=["fenv"]),
            versions=[dict(name="v1alpha1", served=True, storage=True, schema=dict(openAPIV3Schema=schema), subresources=dict(status={}),
                additionalPrinterColumns=[dict(name="Phase", type="string", jsonPath=".status.phase"), dict(name="Namespace", type="string", jsonPath=".status.namespace.name")])]))


def infrastructure(image):
    sa = obj("ServiceAccount", "environment-controller", automountServiceAccountToken=True)
    role = dict(apiVersion="rbac.authorization.k8s.io/v1", kind="ClusterRole", metadata=dict(name="forge-environment-controller"), rules=[
        dict(apiGroups=[GROUP], resources=["forgeenvironments"], verbs=["get", "list", "watch", "patch", "update"]),
        dict(apiGroups=[GROUP], resources=["forgeenvironments/status", "forgeenvironments/finalizers"], verbs=["get", "patch", "update"]),
        dict(apiGroups=[""], resources=["namespaces"], verbs=["get", "list", "watch", "create", "patch", "delete"]),
        dict(apiGroups=[""], resources=["resourcequotas", "limitranges", "serviceaccounts"], verbs=["get", "list", "watch", "create", "patch"]),
        dict(apiGroups=[""], resources=["persistentvolumeclaims", "persistentvolumes"], verbs=["get", "list"])])
    binding = dict(apiVersion="rbac.authorization.k8s.io/v1", kind="ClusterRoleBinding", metadata=dict(name="forge-environment-controller"),
        roleRef=dict(apiGroup="rbac.authorization.k8s.io", kind="ClusterRole", name=role["metadata"]["name"]),
        subjects=[dict(kind="ServiceAccount", name=sa["metadata"]["name"], namespace=NAMESPACE)])
    lease_role = obj("Role", "environment-leader", rules=[
        dict(apiGroups=["coordination.k8s.io"], resources=["leases"], verbs=["get", "list", "watch", "create", "update", "patch"]),
        dict(apiGroups=[""], resources=["events"], verbs=["create","patch"])])
    lease_role["apiVersion"]="rbac.authorization.k8s.io/v1"
    lease_binding = obj("RoleBinding", "environment-leader", roleRef=dict(apiGroup="rbac.authorization.k8s.io", kind="Role", name="environment-leader"), subjects=binding["subjects"])
    lease_binding["apiVersion"]="rbac.authorization.k8s.io/v1"
    c = container("controller", image, 65532, "256Mi", "500m", command=["/forge-environment-controller"],
        env=[secret_env("FORGE_ENVIRONMENT_DATABASE_URL", "environment-db", "url")],
        readinessProbe=dict(httpGet=dict(path="/readyz", port=8084), periodSeconds=3),
        livenessProbe=dict(httpGet=dict(path="/healthz", port=8084), periodSeconds=10))
    c["securityContext"]["readOnlyRootFilesystem"]=True
    d=deployment("environment-controller", [c], serviceAccountName="environment-controller")
    # This privileged controller alone needs its scoped Kubernetes API token.
    d["spec"]["template"]["spec"]["automountServiceAccountToken"]=True
    return [sa,role,binding,lease_role,lease_binding,d]
