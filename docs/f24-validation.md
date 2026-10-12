# F24 verification — 12 October 2026

F24 adds model execution on top of the frozen context and the corpus tool. [D39](decision-log.md#d39--f24-model-execution-and-evidence-based-results) records the backend and the secret boundary. The [model guide](model-execution.md) explains both.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/model ./internal/mcp ./internal/execution ./cmd/forge-worker
```

`TestFixtureRecordsEvidenceOrUncertainty` stores an `insufficient_evidence` report and an `answered` report. The answered report cites one evidence ID, names `google/gemma-4-26b-a4b-it:free`, uses provider `fixture`, and records token usage. `TestOpenRouterKeepsTheKeyOutOfTheBody` checks a local HTTP fixture: the key is the Authorization header, the JSON body does not contain it, and a failed response does not echo it. `TestWorkflowPersistsCitedReport` runs retrieval and generation in the SDK test suite and keeps the cited report as the workflow result.

No live OpenRouter call was made. No public run route or product report table was added.

Documentation checks reported 60 Markdown documents and 427 relative links. The PowerShell checker parsed 8 scripts and 30 documented PowerShell blocks.
