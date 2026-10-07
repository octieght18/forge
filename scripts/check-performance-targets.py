"""Assess recorded F08/F10 summaries against owner-approved local test limits."""
import argparse
import json
import math
from pathlib import Path

DEFAULT_TARGETS = Path(__file__).resolve().parents[1] / "docs/performance-targets.json"


def number(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or value < 0:
        raise ValueError("expected a finite nonnegative metric")
    return value


def count(value):
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        raise ValueError("expected a nonnegative integer count")
    return value


def limit(metric, observed, maximum):
    return {"metric": metric, "observed": number(observed), "maximum": number(maximum), "passed": observed <= maximum}


def assess_baseline(summary, targets):
    if summary["protocol_complete"] is not True:
        raise ValueError("baseline protocol incomplete or failed")
    checks = []
    groups = summary["groups"]
    for phase, maximum in targets["baseline_limits_ms"].items():
        group = groups[phase]
        trials = targets["baseline_trials"][phase]
        success_field = "expected_failures" if phase == "occupied_port_detection" else "successful_trials"
        if group["sample_complete"] is not True or any(count(group[key]) != trials for key in ("expected_trials", "observed_trials", success_field)):
            raise ValueError("baseline trial counts do not match the approved protocol")
        checks.append(limit(phase + ".max_ms", group["max_ms"], maximum))
    for phase, group in groups.items():
        checks.append(limit(phase + ".unexpected_failures", count(group["unexpected_failures"]), targets["unexpected_error_limit"]))
    return checks


def assess_profile(summary, targets):
    levels = summary["levels"]
    if sorted(count(level["clients"]) for level in levels) != targets["profile_clients"]:
        raise ValueError("profile must contain exactly one level for one client and one for two clients")
    checks = []
    for level in levels:
        if level["smoke"] is not False or number(level["elapsed_seconds_including_drain"]) < targets["profile_measure_min_seconds"]:
            raise ValueError("short/smoke observations cannot satisfy the approved full-load target")
        if count(level["request_latency"]["count"]) == 0 or count(level["journey_latency"]["count"]) == 0:
            raise ValueError("profile has no measured requests/journeys")
        name = f"clients-{level['clients']}"
        checks.append(limit(name + ".request_p95_ms", level["request_latency"]["p95_ms"], targets["profile_request_p95_ms"]))
        for key in ("failed_requests", "failed_journeys", "transport_errors", "all_phase_failed_requests", "all_phase_failed_journeys"):
            checks.append(limit(name + "." + key, count(level[key]), targets["unexpected_error_limit"]))
    return checks


def assess(targets, baseline=None, profile=None):
    if baseline is None and profile is None:
        raise ValueError("supply at least one summary")
    checks = []
    if baseline is not None:
        checks.extend(assess_baseline(baseline, targets))
    if profile is not None:
        checks.extend(assess_profile(profile, targets))
    return {"target_profile": targets["profile"], "assessment": "recorded observations; local repeat-test limits, not a production SLO", "passed": all(c["passed"] for c in checks), "checks": checks}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--targets", type=Path, default=DEFAULT_TARGETS)
    parser.add_argument("--baseline-summary", type=Path)
    parser.add_argument("--profile-summary", type=Path)
    args = parser.parse_args()
    read = lambda path: json.loads(path.read_text(encoding="utf-8-sig")) if path else None
    try:
        result = assess(read(args.targets), read(args.baseline_summary), read(args.profile_summary))
    except (ValueError, KeyError, TypeError, OSError) as error:
        print(json.dumps({"passed": False, "assessment": "invalid or inapplicable evidence", "error": str(error)}))
        raise SystemExit(2)
    print(json.dumps(result, indent=2))
    if not result["passed"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
