import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("profile_summary", Path(__file__).with_name("profile-summary.py"))
summary = importlib.util.module_from_spec(spec)
spec.loader.exec_module(summary)


class SummaryTests(unittest.TestCase):
    def test_nearest_rank_and_outlier_retained(self):
        d = summary.distribution([1_000_000, 2_000_000, 100_000_000])
        self.assertEqual((d["p50_ms"], d["p95_ms"], d["max_ms"]), (2, 100, 100))
        self.assertEqual(summary.distribution([]), {"count": 0})

    def test_failures_and_phase_denominators(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "conditions.json").write_text(json.dumps({"clients": 1, "smoke": True, "phases": [{"phase": "measure", "elapsed_seconds_including_drain": 2}]}))
            rows = [dict(kind="request", phase="warmup", elapsed_ns=10, success=False, operation="create_workload", status=500),
                    dict(kind="request", phase="measure", elapsed_ns=20, success=True, operation="create_workload", status=201),
                    dict(kind="request", phase="measure", elapsed_ns=30, success=False, operation="create_version", transport_error=True),
                    dict(kind="journey", phase="measure", elapsed_ns=60, success=False)]
            (root / "requests.jsonl").write_text("".join(json.dumps(r) + "\n" for r in rows))
            result = summary.summarize(root)
            self.assertEqual(result["request_error_fraction"], .5)
            self.assertEqual(result["journey_error_fraction"], 1)
            self.assertEqual(result["transport_errors"], 1)
            self.assertEqual(result["status_counts"], {"0": 1, "201": 1})
            self.assertEqual(result["completed_requests_per_second"], 1)
            self.assertEqual(result["rows_by_phase"]["warmup"], 1)
            self.assertEqual(result["all_phase_failed_requests"], 2)
            self.assertEqual(result["all_phase_failed_journeys"], 1)


if __name__ == "__main__":
    unittest.main()
