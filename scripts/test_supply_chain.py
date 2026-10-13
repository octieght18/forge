"""Supply-chain decisions stay explicit when scanners and signers are absent."""
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("supply_chain", Path(__file__).with_name("supply-chain.py"))
supply_chain = importlib.util.module_from_spec(spec)
spec.loader.exec_module(supply_chain)


class SupplyChainTests(unittest.TestCase):
    def test_module_sbom_uses_go_sum_versions_once(self):
        text = "\n".join([
            "github.com/example/mod v1.2.3 h1:abc",
            "github.com/example/mod v1.2.3/go.mod h1:def",
            "github.com/example/mod v1.2.4 h1:ghi",
        ])
        components = supply_chain.module_components(text)
        self.assertEqual([item["version"] for item in components], ["v1.2.3", "v1.2.4"])
        document = supply_chain.module_sbom(components)
        self.assertEqual(document["bomFormat"], "CycloneDX")
        self.assertNotIn("h1:", json.dumps(document))

    def test_missing_tools_do_not_claim_a_clean_scan_or_signature(self):
        decision = supply_chain.plan(set(), "", "")
        self.assertEqual(decision["module_sbom"], "run")
        self.assertEqual(decision["image_sbom"], "not-run")
        self.assertEqual(decision["image_scan"], "not-run")
        self.assertEqual(decision["signed_image"], "not-run")
        self.assertEqual(decision["clean_scan"], "not-claimed")
        self.assertIn("does not prove safety", decision["signature_meaning"])

    def test_signer_without_an_image_does_not_sign(self):
        decision = supply_chain.plan({"cosign", "grype", "syft"}, "", "")
        self.assertEqual(decision["signed_image"], "not-run")
        self.assertEqual(decision["image_scan"], "not-run")

    def test_repository_module_sbom_is_not_empty(self):
        root = Path(__file__).resolve().parents[1]
        components = supply_chain.module_components((root / "go.sum").read_text(encoding="utf-8"))
        self.assertGreater(len(components), 10)


if __name__ == "__main__":
    unittest.main()
