# F08 baseline procedure

D21 accepts an initial baseline of the **existing native stack and authenticated registration path**. It does not measure environment provisioning or human effort. The scope is five retained-data starts, ten sequential registration/read journeys and one occupied-port detection/recovery case. All services remain native in Ubuntu WSL; existing data stays intact.

## Repeat the system measurements

Prerequisites: follow the [native setup guide](local-stack.md). PostgreSQL, Java, Python, the pinned Keycloak archive and Go toolchain must already be installed, the stack must have completed an initial successful start, and the example corpus policy must be approved. Use PowerShell 7 on Windows. Stop other benchmarks; note any remaining host activity. The protocol intentionally uses existing build/download caches, without clearing or prewarming them between trials.

From the repository root:

```powershell
.\scripts\measure-baseline.ps1 -OutputDirectory .\tmp\f08\my-run
wsl.exe -d Ubuntu --exec python3 /mnt/c/Users/Owner/Downloads/Code/forge/scripts/baseline/summarize.py /mnt/c/Users/Owner/Downloads/Code/forge/tmp/f08/my-run
```

Change both paths if the checkout is elsewhere. Choose a new output directory for each attempt; the harness refuses to overwrite one. The defaults are five startup and ten registration trials. Shorter samples are useful for troubleshooting but do not satisfy D21. Summarization exits nonzero for incomplete or failed required samples. Keep failed attempts and all outliers alongside the final complete run; explain any instrumentation correction rather than silently discarding results.

The harness performs these steps:

1. Capture CPU, Windows/WSL/kernel/runtime versions, memory, source commit and harness hash, initial service status and measurement boundaries. It records wall-clock timestamps in UTC and durations with a monotonic stopwatch.
2. Repeat normal `local-stack.ps1 stop` followed by `up` five times. Time the whole `up`: WSL entry, three Go builds, explicit migrations/grants, database/Keycloak/API readiness and Windows loopback verification. Time stops separately. Installation/download, fresh cluster/realm initialization and human effort are excluded. These starts are retained-data restarts; the first is not a cold installation.
3. Obtain a real Keycloak authorization-code/S256 PKCE access token through the existing helper, using private local demo credentials in memory. Record this automated setup separately. It is not human login timing.
4. For each of ten unique synthetic workloads, issue POST workload, POST immutable version, GET workload and GET version through the Windows loopback client, sequentially. Check status, request ID and matching readback IDs. Record each HTTP duration and the full journey. The journey includes construction/parsing/checks; per-request timing ends after response body receipt. A shared client session allows connection reuse. The first sample may include JWKS/client cache warmup; retain it. No concurrent load, model call or workflow execution occurs.
5. Stop normally. An isolated native socket fixture occupies API port 8081. Time `up` until it fails with the known address-in-use diagnostic. Release only that fixture, then time `up` through readiness again. Diagnose/release time is not included in recovery duration. No database deletion occurs. Leave the stack running after successful recovery.

Raw JSONL includes every completed timed attempt, failures, phase/trial, command IDs and safe status/fixture IDs. Metadata maps IDs to reproducible commands and expected sample counts. Tokens, passwords, environment variables, DSNs and HTTP bodies are omitted from published results. Local token files remain private in the native state directory. Do not publish the state directory or arbitrary systemd output.

If `up` fails unexpectedly, inspect `local-stack.ps1 status` and private systemd diagnostics using the native guide. Retain the raw failed timing. Release the fixture only if this harness created it; never kill an unrelated port owner. Repeat into a new directory after correcting the cause. Normal `stop` retains identities and records; `purge` is not part of measurement or recovery.

## Human onboarding worksheet

Use the [unmeasured worksheet](baseline-human-worksheet.json) to record a person's work separately. Begin before reading setup instructions; end after the first authenticated version read. Record prerequisites already present, documentation/version, manual commands, clicks, credential setup, mistakes, assistance, active typing/decision time and system wait time. Include package/download/bootstrap time if measuring fresh onboarding. Do not substitute automation wall time for human effort. Existing observations do not measure this journey.

The current manual path is: verify/install the native prerequisites from the guide; run `local-stack.ps1 up`; inspect status/readiness; run `local-stack.ps1 login`; open the provided loopback URL; authenticate in Keycloak; load the private token into a local client; submit the example workload/version and read them back. The [registration guide](registration-api.md) documents routes and the [contract examples](../internal/contract/examples/create-version.json) supply the payload. Never paste a token into the worksheet. The harness replaces typing and form interaction only for repeatable system timings, without claiming observed steps saved.

## Later manual versus self-service provisioning

The D08 comparison remains **ten manual and ten self-service provisioning trials** when provisioning exists. Registration is not environment provisioning. Define an equivalent target environment, workload version, owner, policy and resource settings before those trials. Freeze hardware/software and prerequisites, classify cached versus fresh bootstrap equally, alternate manual/automated order where practical, and retain all failures and raw timings.

For each manual trial, start when the operator receives the same validated request, record each command/approval and active effort, and stop only after equivalent developer-accessible readiness. For each self-service trial, start at request submission and stop at that same readiness criterion. Record reconciliation attempts, resource IDs and cleanup, failure trigger/detection, manual intervention and recovery origin. Separate machine wait from human effort; do not exclude bootstrap on only one side. Report sample counts, success/failure, median/range and observed manual steps for both paths. Consult the owner on the exact provisioning/runtime design and numerical targets before claiming improvement.

## Interpretation

Five startup and ten registration samples are descriptive, sequential observations on one PC. Report milliseconds or seconds with sample counts and median/min/max. Do not extrapolate throughput, concurrent capacity, percentiles, production reliability or an SLO from this sample. Occupied-port recovery is orderly and programmatic, not crash recovery or an operational recovery objective. D23 accepts the [Balanced local repeat-test limits](performance-targets.md). After generating a new comparable summary, run `python3 scripts/check-performance-targets.py --baseline-summary /absolute/path/to/new-run/summary.json` to assess them.

See the [published report](f08-baseline.md) for actual observations and retained raw artifacts.
