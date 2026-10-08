"""Guard the deployment's trust boundaries before any cluster is touched."""
import json
from pathlib import Path
import unittest
import resources


class BoundaryTests(unittest.TestCase):
    def setUp(self):
        self.lock = json.loads((resources.HERE / "images.json").read_text())
        self.objects = resources.foundation() + resources.database(self.lock) + resources.apps(self.lock, "forge-api:test") + [resources.migration("forge-api:test", self.lock["postgres"])]

    def test_no_host_exposure_or_service_account_credentials(self):
        for item in self.objects:
            if item["kind"] == "Service":
                self.assertEqual(item["spec"]["type"], "ClusterIP")
                self.assertNotIn("externalIPs", item["spec"])
            if item["kind"] not in ("Deployment", "Job"):
                continue
            pod = item["spec"]["template"]["spec"]
            self.assertFalse(pod["automountServiceAccountToken"])
            self.assertFalse(pod.get("hostNetwork", False))
            for container in pod["containers"] + pod.get("initContainers", []):
                context = container["securityContext"]
                self.assertTrue(context["runAsNonRoot"])
                self.assertFalse(context["allowPrivilegeEscalation"])
                self.assertEqual(context["capabilities"]["drop"], ["ALL"])
                self.assertTrue(container["resources"]["limits"])
                self.assertFalse(any("hostPort" in port for port in container.get("ports", [])))

    def test_runtime_does_not_receive_migration_or_dispatcher_secret(self):
        api = next(o for o in self.objects if o["kind"] == "Deployment" and o["metadata"]["name"] == "api")
        names = {env["valueFrom"]["secretKeyRef"]["name"] for c in api["spec"]["template"]["spec"]["containers"] for env in c.get("env", []) if "valueFrom" in env}
        self.assertEqual(names, {"api-db"})
        config = {env["name"]: env["value"] for env in api["spec"]["template"]["spec"]["containers"][0]["env"] if "value" in env}
        self.assertEqual(config["FORGE_HTTP_ADDR"], "127.0.0.1:8081")
        self.assertEqual(config["FORGE_OIDC_ISSUER"], "http://127.0.0.1:8082/realms/forge")
        sidecar = api["spec"]["template"]["spec"]["initContainers"][0]
        self.assertEqual(sidecar["restartPolicy"], "Always")
        self.assertEqual(sidecar["startupProbe"]["httpGet"]["path"], "/proxy-ready")

    def test_immutable_dependencies_and_retained_storage(self):
        for image in self.lock.values():
            self.assertRegex(image, r"@sha256:[a-f0-9]{64}$")
        pv = next(o for o in self.objects if o["kind"] == "PersistentVolume")
        self.assertEqual(pv["spec"]["persistentVolumeReclaimPolicy"], "Retain")
        self.assertEqual(pv["spec"]["hostPath"]["path"], "/forge-data/postgres")
        self.assertNotIn("namespace", pv["metadata"])

    def test_build_context_is_an_allowlist(self):
        ignore = (resources.HERE.parents[1] / ".dockerignore").read_text().splitlines()
        rules = [line for line in ignore if line and not line.startswith("#")]
        self.assertEqual(rules[0], "**")
        self.assertFalse(any(rule.startswith("!.git") or rule.startswith("!docs") for rule in rules))


if __name__ == "__main__":
    unittest.main()
