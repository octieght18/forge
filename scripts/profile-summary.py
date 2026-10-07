"""Summarize F10 JSONL observations without removing errors or warmup rows."""
import argparse
import collections
import gzip
import json
import math
from pathlib import Path


def distribution(values):
    ordered = sorted(values)
    if not ordered:
        return {"count": 0}
    rank = lambda p: ordered[max(0, math.ceil(len(ordered) * p) - 1)] / 1_000_000
    return {"count": len(ordered), "min_ms": ordered[0] / 1_000_000,
            "p50_ms": rank(.50), "p95_ms": rank(.95), "p99_ms": rank(.99),
            "max_ms": ordered[-1] / 1_000_000}


def summarize(directory):
    directory = Path(directory)
    conditions = json.loads((directory / "conditions.json").read_text())
    raw = directory / "requests.jsonl"
    opener = open
    if not raw.exists():
        raw = directory / "requests.jsonl.gz"
        opener = gzip.open
    with opener(raw, "rt", encoding="utf-8") as stream:
        rows = [json.loads(line) for line in stream]
    for row in rows:
        if row["kind"] not in ("request", "journey") or row["elapsed_ns"] < 0:
            raise ValueError("invalid observation")
    measured = [row for row in rows if row["phase"] == "measure"]
    requests = [row for row in measured if row["kind"] == "request"]
    journeys = [row for row in measured if row["kind"] == "journey"]
    if not requests or not journeys:
        raise ValueError("no measured requests/journeys")
    failures = sum(not row["success"] for row in requests)
    failed_journeys = sum(not row["success"] for row in journeys)
    elapsed = next(p["elapsed_seconds_including_drain"] for p in conditions["phases"] if p["phase"] == "measure")
    if elapsed <= 0:
        raise ValueError("invalid measurement duration")
    operations = collections.defaultdict(list)
    for row in requests:
        operations[row["operation"]].append(row["elapsed_ns"])
    return {"clients": conditions["clients"], "smoke": conditions["smoke"],
            "elapsed_seconds_including_drain": elapsed,
            "request_latency": distribution([r["elapsed_ns"] for r in requests]),
            "journey_latency": distribution([r["elapsed_ns"] for r in journeys]),
            "operation_latency": {op: distribution(v) for op, v in sorted(operations.items())},
            "status_counts": dict(sorted(collections.Counter(str(r.get("status", 0)) for r in requests).items())),
            "transport_errors": sum(r.get("transport_error", False) for r in requests),
            "failed_requests": failures, "request_error_fraction": failures / len(requests),
            "failed_journeys": failed_journeys, "journey_error_fraction": failed_journeys / len(journeys),
            "all_phase_failed_requests": sum(r["kind"] == "request" and not r["success"] for r in rows),
            "all_phase_failed_journeys": sum(r["kind"] == "journey" and not r["success"] for r in rows),
            "completed_requests_per_second": len(requests) / elapsed,
            "successful_journeys_per_second": (len(journeys) - failed_journeys) / elapsed,
            "rows_by_phase": dict(sorted(collections.Counter(r["phase"] for r in rows).items()))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    args = parser.parse_args()
    results = [summarize(p) for p in sorted(args.directory.glob("clients-*")) if p.is_dir()]
    if not results:
        raise SystemExit("no client-level observations found")
    print(json.dumps({"quantile_method": "nearest rank; all measured outcomes included", "levels": results}, indent=2))


if __name__ == "__main__":
    main()
