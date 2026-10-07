import copy
import importlib.util
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("targets", ROOT / "scripts/check-performance-targets.py")
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class TargetTests(unittest.TestCase):
    def setUp(self):
        self.targets = json.loads((ROOT / "docs/performance-targets.json").read_text())
        self.baseline = json.loads((ROOT / "docs/baselines/f08-2026-10-07/summary.json").read_text())
        self.profile = json.loads((ROOT / "docs/validation/f10-registration-summary.json").read_text())

    def test_recorded_observations_pass(self):
        self.assertTrue(checker.assess(self.targets, self.baseline, self.profile)["passed"])

    def test_threshold_equality_and_single_exceedance(self):
        for phase, maximum in self.targets["baseline_limits_ms"].items():
            with self.subTest(phase=phase):
                data = copy.deepcopy(self.baseline)
                data["groups"][phase]["max_ms"] = maximum
                self.assertTrue(checker.assess(self.targets, baseline=data)["passed"])
                data["groups"][phase]["max_ms"] = maximum + .001
                self.assertFalse(checker.assess(self.targets, baseline=data)["passed"])
        self.profile["levels"][1]["request_latency"]["p95_ms"] = 5
        self.assertTrue(checker.assess(self.targets, profile=self.profile)["passed"])
        self.profile["levels"][1]["request_latency"]["p95_ms"] = 5.001
        self.assertFalse(checker.assess(self.targets, profile=self.profile)["passed"])

    def test_errors_including_warmup_fail(self):
        self.baseline["groups"]["automated_pkce_setup"]["unexpected_failures"] = 1
        self.assertFalse(checker.assess(self.targets, baseline=self.baseline)["passed"])
        self.profile["levels"][0]["all_phase_failed_requests"] = 1
        self.assertFalse(checker.assess(self.targets, profile=self.profile)["passed"])

    def test_incomplete_and_wrong_trial_counts_rejected(self):
        self.baseline["protocol_complete"] = False
        with self.assertRaises(ValueError):
            checker.assess(self.targets, baseline=self.baseline)
        self.baseline["protocol_complete"] = True
        self.baseline["groups"]["retained_startup"]["observed_trials"] = 4
        with self.assertRaises(ValueError):
            checker.assess(self.targets, baseline=self.baseline)

    def test_smoke_short_and_missing_client_levels_rejected(self):
        for change in (lambda x: x[0].update(smoke=True), lambda x: x[0].update(elapsed_seconds_including_drain=.3), lambda x: x.pop()):
            data = copy.deepcopy(self.profile)
            change(data["levels"])
            with self.assertRaises(ValueError):
                checker.assess(self.targets, profile=data)

    def test_invalid_metrics_and_missing_evidence_rejected(self):
        for value in (float("nan"), float("inf"), -1, True, "5"):
            self.profile["levels"][0]["request_latency"]["p95_ms"] = value
            with self.assertRaises(ValueError):
                checker.assess(self.targets, profile=self.profile)
        with self.assertRaises(ValueError):
            checker.assess(self.targets)


if __name__ == "__main__":
    unittest.main()
