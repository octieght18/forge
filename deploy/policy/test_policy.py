"""Reject privileged workload specs from the kind resource generator."""
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "kubernetes"))
import resources


def pod_specs(obj):
    kind = obj.get("kind")
    spec = obj.get("spec") or {}
    if kind == "Pod":
        yield spec
    elif kind in {"Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job"}:
        yield spec.get("template", {}).get("spec", {})
    elif kind == "CronJob":
        yield spec.get("jobTemplate", {}).get("spec", {}).get("template", {}).get("spec", {})


def findings(obj):
    found = []
    for spec in pod_specs(obj):
        if not spec:
            continue
        if spec.get("hostNetwork") or spec.get("hostPID") or spec.get("hostIPC"):
            found.append("host namespace")
        for volume in spec.get("volumes") or []:
            if "hostPath" in volume:
                found.append("hostPath")
        pod_safe = spec.get("securityContext") or {}
        containers = (spec.get("containers") or []) + (spec.get("initContainers") or []) + (spec.get("ephemeralContainers") or [])
        for container in containers:
            security = container.get("securityContext") or {}
            if security.get("privileged"):
                found.append("privileged")
            if security.get("allowPrivilegeEscalation") is not False:
                found.append("allowPrivilegeEscalation")
            if security.get("runAsNonRoot") is not True and pod_safe.get("runAsNonRoot") is not True:
                found.append("runAsNonRoot")
            caps = security.get("capabilities") or {}
            if "ALL" not in (caps.get("drop") or []):
                found.append("capabilities")
            if caps.get("add"):
                found.append("added capability")
            if any("hostPort" in port for port in container.get("ports") or []):
                found.append("hostPort")
    return found


class PolicyTests(unittest.TestCase):
    def test_kind_resources_are_not_privileged(self):
        lock = json.loads((resources.HERE / "images.json").read_text())
        objects = resources.foundation() + resources.database(lock) + resources.apps(lock, "forge-api:test") + [resources.migration("forge-api:test", lock["postgres"])]
        for obj in objects:
            self.assertEqual(findings(obj), [], obj.get("kind"))

    def test_privileged_fixture_is_rejected(self):
        unsafe = {"kind": "Pod", "spec": {"hostNetwork": True, "containers": [{"name": "app", "securityContext": {"privileged": True, "allowPrivilegeEscalation": True, "capabilities": {"add": ["SYS_ADMIN"]}}, "ports": [{"hostPort": 80}]}], "volumes": [{"name": "host", "hostPath": {"path": "/var/run"}}]}}
        self.assertIn("privileged", findings(unsafe))
        self.assertIn("host namespace", findings(unsafe))


if __name__ == "__main__":
    unittest.main()
