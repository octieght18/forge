# F14 verification — 9 October 2026

F14 adds the developer CLI in [developer-cli.md](developer-cli.md). [D28](decision-log.md#d28--f14-developer-cli-over-the-registration-api) records why `deploy` and `delete` stay inside the current registration API. No workload deletion, public environment route, run endpoint, or Kubernetes credential was added.

## Local checks

Linux Go 1.27.1 and Windows Go 1.27.1 both passed `go test -count=1 ./internal/cli`. The Linux race run `go test -race -count=1 ./internal/cli` passed. `go vet ./internal/cli ./cmd/forge` passed. The tests use an in-process API and check:

- `register`, `deploy`, and `status` print JSON on stdout with the server `request_id`
- `deploy` reports `execution: not_started` and `environment: not_requested` and sends the spec unchanged
- `delete` exits 1 with the API 405, `method_not_allowed`, a hint, and the operation ID
- invalid names, specs, IDs, expired tokens, and credentialed API URLs fail before any HTTP call
- an API error that echoes the access token is suppressed
- redirects are not followed and the token is not sent onward

PowerShell parsing passed 7 scripts and 25 documented blocks, including [cli-examples.ps1](../scripts/cli-examples.ps1) and the CLI examples in the developer guide and quickstart. The documentation check passed 41 Markdown documents and 304 relative links.

On this PC the native stack was started from the existing Ubuntu installation and the same journey was run inside WSL as the developer `ahmad`, using a real Keycloak authorization-code login. Register returned 201, deploy returned 201 with `execution: not_started` and `environment: not_requested`, status read that version, delete exited 1 with `method_not_allowed`, and a following status still returned the workload. The created workload was `ed246da4-f50a-4bbe-bbc0-95bb7393a553` and the version was `345b8e12-bed2-42d3-be71-70e6a2fda2c5`. The native and Kubernetes CI jobs also run `scripts/cli-examples.ps1` with the real `ahmad` token. Hosted results are still recorded on the F14 card after the commit is published.

## Material limits

`deploy` stores an immutable version. It does not provision a namespace, start a worker, or call a model. `delete` does not remove the workload or version; the current API answers 405 and the following `status` still reads the record. Register and deploy are not idempotent: repeating them can create another workload or version. The access token stays in the private login file, expires after five minutes, and is omitted from command output. Operator environment apply and delete remain [environment commands](execution-environments.md).
