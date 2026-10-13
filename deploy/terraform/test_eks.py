"""The EKS module is one target, pinned, and free of credentials."""
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent / "eks"
DOCS = Path(__file__).resolve().parents[2] / "docs" / "terraform-eks.md"


class EKSModuleTests(unittest.TestCase):
    def setUp(self):
        self.text = "\n".join(path.read_text(encoding="utf-8") for path in sorted(ROOT.glob("*.tf")))

    def test_one_aws_target_with_pinned_providers(self):
        self.assertIn('version = "6.68.0"', self.text)
        self.assertIn('required_version = "1.16.5"', self.text)
        self.assertIn("aws_eks_cluster", self.text)
        self.assertNotIn("google_", self.text)
        self.assertNotIn("azurerm_", self.text)
        self.assertNotIn("AdministratorAccess", self.text)
        self.assertNotIn("aws_access_key", self.text)
        self.assertNotIn("secret_key", self.text)

    def test_minimal_iam_and_private_endpoint(self):
        self.assertIn("AmazonEKSClusterPolicy", self.text)
        self.assertIn("AmazonEKSWorkerNodePolicy", self.text)
        self.assertIn("AmazonEKS_CNI_Policy", self.text)
        self.assertIn("AmazonEC2ContainerRegistryReadOnly", self.text)
        self.assertIn("endpoint_public_access  = false", self.text)

    def test_local_backend_and_no_state_file(self):
        self.assertIn('backend "local"', self.text)
        self.assertFalse(list(ROOT.glob("*.tfstate")))
        self.assertFalse(list(ROOT.glob("*.tfvars")))
        guide = DOCS.read_text(encoding="utf-8")
        self.assertIn("No spend occurred", guide)
        self.assertIn("backend.hcl", guide)
        self.assertIn("G4", guide)


if __name__ == "__main__":
    unittest.main()