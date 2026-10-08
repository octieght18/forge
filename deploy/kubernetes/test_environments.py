import unittest
import environments


class EnvironmentTrustTests(unittest.TestCase):
    def test_controller_has_no_product_write_or_secret_read_permissions(self):
        objects=environments.infrastructure("forge-api:test")
        role=next(o for o in objects if o["kind"]=="ClusterRole")
        for rule in role["rules"]:
            self.assertNotIn("secrets",rule["resources"])
            self.assertNotIn("pods",rule["resources"])
            self.assertNotIn("roles",rule["resources"])
            self.assertNotIn("*",rule["verbs"]+rule["resources"])
        d=next(o for o in objects if o["kind"]=="Deployment")
        p=d["spec"]["template"]["spec"]
        self.assertEqual(p["serviceAccountName"],"environment-controller")
        self.assertTrue(p["automountServiceAccountToken"])
        c=p["containers"][0]
        self.assertEqual(c["env"][0]["valueFrom"]["secretKeyRef"]["name"],"environment-db")
        self.assertTrue(c["securityContext"]["readOnlyRootFilesystem"])
        self.assertFalse(c["securityContext"]["allowPrivilegeEscalation"])

    def test_environment_identity_immutable_and_status_separate(self):
        spec=environments.crd()["spec"]
        self.assertEqual(spec["scope"],"Cluster")
        version=spec["versions"][0]
        self.assertEqual(version["subresources"],{"status":{}})
        schema=version["schema"]["openAPIV3Schema"]
        identity=schema["properties"]["spec"]
        self.assertEqual(identity["x-kubernetes-validations"][0]["rule"],"self == oldSelf")
        self.assertEqual(set(identity["properties"]),{"workloadID","owner","profile"})
        self.assertEqual(identity["properties"]["profile"]["enum"],["small-v1"])
