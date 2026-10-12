# F30 verification — 12 October 2026

F30 publishes the workflow and replay guide. [D45](decision-log.md#d45--f30-workflow-and-replay-guide) records the boundary. The guide is [workflow-replay-guide.md](workflow-replay-guide.md). It includes the F29 sample: 43361 µs for the dependency failure, two retrieval attempts, one compensated evidence row, 12993 µs for the uncertain result, 1816 µs for cancellation, and 535 µs for the replayer. Those figures are not a production SLO.

Checked:

```text
python3 scripts/check-docs.py
pwsh -File scripts/check-powershell.ps1
```

No new runtime behavior was added. The Temporal dev server was not started. A live worker kill remains unrun, as recorded in F29.
