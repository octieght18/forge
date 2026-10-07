#!/usr/bin/env python3
"""Summarize observed system timings; never infer human time or an SLO."""
import argparse
import json
import math
from pathlib import Path
import statistics


def summarize(records, expected):
    result = {"schema_version": 1, "human_effort_seconds": None, "slo_assessment": "not_defined", "groups": {}}
    seen = set()
    for record in records:
        if record["record_id"] in seen:
            raise ValueError("Duplicate raw record ID")
        seen.add(record["record_id"])
        value = record["elapsed_ms"]
        if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or value < 0:
            raise ValueError("Invalid elapsed duration")
        if record["outcome"] not in ("success", "failure", "expected_failure"):
            raise ValueError("Unknown outcome")
    for phase, count in expected.items():
        group = [r for r in records if r["phase"] == phase]
        success = [r["elapsed_ms"] for r in group if r["outcome"] == "success"]
        anticipated = [r["elapsed_ms"] for r in group if r["outcome"] == "expected_failure"]
        failures = sum(r["outcome"] == "failure" for r in group)
        values = anticipated if phase == "occupied_port_detection" else success
        result["groups"][phase] = {
            "expected_trials": count, "observed_trials": len(group), "successful_trials": len(success),
            "expected_failures": len(anticipated), "unexpected_failures": failures,
            "sample_complete": len(group) == count,
            "median_ms": round(statistics.median(values), 3) if values else None,
            "min_ms": round(min(values), 3) if values else None,
            "max_ms": round(max(values), 3) if values else None,
        }
    result["protocol_complete"] = all(g["sample_complete"] and g["unexpected_failures"] == 0 and
                                      (g["expected_failures"] == g["expected_trials"] if phase == "occupied_port_detection" else g["successful_trials"] == g["expected_trials"])
                                      for phase, g in result["groups"].items())
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    options = parser.parse_args()
    metadata = json.loads((options.directory / "metadata.json").read_text(encoding="utf-8-sig"))
    records = [json.loads(line) for line in (options.directory / "raw.jsonl").read_text(encoding="utf-8-sig").splitlines() if line.strip()]
    result = summarize(records, metadata["expected_trials"])
    (options.directory / "summary.json").write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result))
    if not result["protocol_complete"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
