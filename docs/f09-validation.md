# F09 quickstart validation

F09 publishes the initial native quickstart, current API reference, executable examples and repository navigation. It applies the previously accepted real Keycloak/OIDC identity design (D13/D19) and native Windows/Ubuntu deployment (D20). The original card's placeholder-auth checklist is superseded by those already-approved requirements; no fake identity mechanism is introduced.

Published: [quickstart](quickstart.md), [current API reference](api-examples.md), [repository navigation](repository-map.md), [runnable seven-operation client](../scripts/api-examples.ps1) and [OpenAPI](../internal/contract/openapi.json). Documentation explicitly identifies unimplemented run/history/report/evidence routes, real auth, owner/operator boundaries, five-minute token expiry, ETag updates, pagination and non-idempotent registration retries.

## Local clean-checkout evidence

On 7 October 2026, a separate clean Git clone of source revision [ecd3bf57b657d246f97c4f9739d9a7ad0719dbf3](https://github.com/octieght18/forge/commit/ecd3bf57b657d246f97c4f9739d9a7ad0719dbf3) passed the published client script in Windows PowerShell 7 against the existing native WSL stack. Authentication used the actual Keycloak authorization form, S256 code flow, callback helper and signed API token; credentials/tokens stayed private. All seven operation kinds passed, with an additional workload-list page obtained through the returned cursor. The created workload/version read back correctly, the PATCH used the authorized GET's ETag and advanced revision from 1 to 2, and version listing returned the new immutable version. [Safe local results](validation/f09-local-quickstart.json) retain source revision, clean status, operation result and fixture IDs. One synthetic workload/version was added and retained; the live services and existing data were not stopped or deleted.

Four PowerShell scripts and 16 documented PowerShell code blocks parsed successfully. The clean-source Markdown check validated 27 repository documents and 176 relative links. An earlier ad hoc checker incorrectly interpreted a PowerShell cast as a Markdown link inside a code fence; the first repository checker then encountered the same cast in inline-code prose. The final checker excludes both fenced and inline code, correcting those false positives without altering the working API example.

## Clean native CI path

The native CI job starts from the exact hosted checkout, installs native prerequisites and initializes fresh PostgreSQL/Keycloak state on an isolated Ubuntu runner. Its real-login smoke supplies a private token file; the **same published PowerShell client** then exercises all seven operations. Existing stop/restart checks separately verify retained identities/product data. Windows CI parses the script and documented PowerShell blocks; Ubuntu CI checks Markdown links/fences. Formatting/module/vet/build, Windows/Ubuntu tests, Linux race and required PostgreSQL/restart jobs remain.

The exact published commit and successful four-job CI run are added to the F09 card only after completion. No pending hosted result is claimed here.

## Fixes and limits

The quickstart now explains that port 8083 is a temporary helper, why revisiting a callback can fail after success, how to start a fresh login and how to load `access_token` into PowerShell/Postman. Older F03/F04/F06 guides now distinguish their stage context from currently available registration, and the issuer example uses the exact native `127.0.0.1` identity. The client suppresses Bearer tokens, response bodies and cursor query strings from its progress output, and fails early for an absent/expired token.

The Windows clean clone reused installed prerequisites and the existing native services; it is not a new Windows/WSL installation test. Fresh native service initialization is covered on the isolated Ubuntu CI runner, not by deleting this PC's state. Automated Keycloak form/PKCE testing is not a visual Firefox/Postman usability study. Package installation and human onboarding/typing effort remain unmeasured. Local HTTP/Keycloak development mode, no production TLS/HA/backup guarantee, no research/model/MCP/Temporal execution and no provisioning remain explicit boundaries. F08 performance targets are still pending owner review; F09 adds no SLO.
