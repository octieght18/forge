import unittest

from summarize import summarize


class SummaryTests(unittest.TestCase):
    def record(self, number, elapsed, outcome="success", phase="startup"):
        return dict(record_id=str(number), phase=phase, elapsed_ms=elapsed, outcome=outcome)

    def test_median_and_range_preserve_outlier(self):
        report = summarize([self.record(1, 10), self.record(2, 20), self.record(3, 100)], {"startup": 3})
        self.assertEqual(report["groups"]["startup"]["median_ms"], 20)
        self.assertEqual(report["groups"]["startup"]["max_ms"], 100)
        self.assertTrue(report["protocol_complete"])
        self.assertIsNone(report["human_effort_seconds"])

    def test_failed_trials_are_visible_and_not_zero_timings(self):
        report = summarize([self.record(1, 12), self.record(2, 400, "failure")], {"startup": 2})
        self.assertEqual(report["groups"]["startup"]["median_ms"], 12)
        self.assertEqual(report["groups"]["startup"]["unexpected_failures"], 1)
        self.assertFalse(report["protocol_complete"])

    def test_incomplete_and_empty_samples_have_no_fabricated_values(self):
        report = summarize([], {"startup": 5})
        self.assertIsNone(report["groups"]["startup"]["median_ms"])
        self.assertFalse(report["protocol_complete"])

    def test_expected_failure_has_separate_group(self):
        report = summarize([self.record(1, 8, "expected_failure", "occupied_port_detection")], {"occupied_port_detection": 1})
        self.assertTrue(report["protocol_complete"])
        self.assertEqual(report["groups"]["occupied_port_detection"]["expected_failures"], 1)
        self.assertEqual(report["groups"]["occupied_port_detection"]["median_ms"], 8)

    def test_duplicates_and_invalid_durations_rejected(self):
        with self.assertRaises(ValueError):
            summarize([self.record(1, 10), self.record(1, 20)], {"startup": 2})
        for value in (float("nan"), float("inf"), -1, True, "10"):
            with self.assertRaises(ValueError):
                summarize([self.record(1, value)], {"startup": 1})


if __name__ == "__main__":
    unittest.main()
