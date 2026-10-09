# Current API and runnable examples

Base URL: **`http://127.0.0.1:8081`**. [Start the native stack and log in](quickstart.md) first. All product operations require a verified Keycloak API access token; health endpoints are public. The [OpenAPI document](../internal/contract/openapi.json) includes future run/history/report/evidence operations as a contract, but **only these seven product operations are implemented**:

| Operation | Method and path | Successful response |
|---|---|---|
| Register workload | `POST /api/v1/workloads` | 201; workload, `Location`, strong ETag |
| List workloads | `GET /api/v1/workloads` | 200; `items`, `next_cursor` |
| Read workload | `GET /api/v1/workloads/{workload_id}` | 200; workload and strong ETag |
| Update workload metadata | `PATCH /api/v1/workloads/{workload_id}` | 200; workload and new ETag; requires `If-Match` |
| Register immutable version | `POST /api/v1/workloads/{workload_id}/versions` | 201; version and `Location` |
| List versions | `GET /api/v1/workloads/{workload_id}/versions` | 200; `items`, `next_cursor` |
| Read immutable version | `GET /api/v1/workloads/{workload_id}/versions/{version_id}` | 200; version |

Server-issued identifiers are fields **`workload_id`** and **`version_id`**, not `id`. Names must be 1–63 lowercase slug characters, unique within the owner. Versions are immutable; no delete, owner transfer, version update or run endpoint is currently available. The [developer CLI](developer-cli.md) calls these same seven operations: `deploy` records a version, and `delete` reports the API's 405 rather than adding a deletion API. Asynchronous environment operations are a separate accepted-work API; see [provisioning operations](provisioning-operations.md).

## Complete example

The executable [PowerShell example](../scripts/api-examples.ps1) makes the requests below, checks expected statuses, readback IDs and revision advancement, and follows a cursor when present. It creates one retained synthetic workload/version per invocation. It uses the validated [version payload](../internal/contract/examples/create-version.json) and does not call a model.

For manual requests, in PowerShell 7 from the repository root:

```powershell
$tokenFile = '\\wsl.localhost\Ubuntu\home\owner\.local\share\forge-native\tokens\access.json'
$token = (Get-Content -LiteralPath $tokenFile -Raw | ConvertFrom-Json).access_token
$auth = @{ Authorization = "Bearer $token" }
$base = 'http://127.0.0.1:8081'

# 1. Register a uniquely named workload: 201, Location and ETag.
$body = @{ name=('example-' + [Guid]::NewGuid().ToString('N')); description='Read-only research example' } | ConvertTo-Json
$createdResponse = Invoke-WebRequest "$base/api/v1/workloads" -Method POST `
    -Headers $auth -ContentType 'application/json' -Body ([Text.Encoding]::UTF8.GetBytes($body))
$workload = $createdResponse.Content | ConvertFrom-Json
$path = "$base/api/v1/workloads/$($workload.workload_id)"

# 2. Read the workload and retain the complete quoted ETag: 200.
$readResponse = Invoke-WebRequest $path -Headers $auth
$etag = [string]($readResponse.Headers['ETag'] | Select-Object -First 1)

# 3. Update metadata against that revision: 200 and a new ETag.
$patchHeaders = @{} + $auth
$patchHeaders['If-Match'] = $etag
$patch = @{ description='Updated description' } | ConvertTo-Json
$updated = Invoke-RestMethod $path -Method PATCH -Headers $patchHeaders `
    -ContentType 'application/json' -Body ([Text.Encoding]::UTF8.GetBytes($patch))

# 4. List and optionally follow the opaque cursor: 200.
$page = Invoke-RestMethod "$base/api/v1/workloads?limit=1" -Headers $auth
if ($page.next_cursor) {
    $cursor = [Uri]::EscapeDataString($page.next_cursor)
    $nextPage = Invoke-RestMethod "$base/api/v1/workloads?limit=1&cursor=$cursor" -Headers $auth
}

# 5. Register the fixed research spec as an immutable version: 201.
$spec = Get-Content -LiteralPath './internal/contract/examples/create-version.json' -Raw
$version = Invoke-RestMethod "$path/versions" -Method POST -Headers $auth `
    -ContentType 'application/json' -Body ([Text.Encoding]::UTF8.GetBytes($spec))

# 6. Read that exact version: 200.
$savedVersion = Invoke-RestMethod "$path/versions/$($version.version_id)" -Headers $auth

# 7. List its parent's versions: 200.
$versions = Invoke-RestMethod "$path/versions?limit=20" -Headers $auth
```

For an existing workload, begin with its authorized GET and ETag; do not POST again to look it up. In Postman, import the OpenAPI JSON or enter these method/URL/body combinations, set **Authorization → Bearer Token**, and add `Content-Type: application/json` for POST/PATCH and the exact `If-Match` value for PATCH. Use the **access token value**, not the entire token JSON, browser cookie, callback code or ID token. Postman-imported future run operations still return 404 until implemented.

## Identity, pages and retries

Ownership comes from signed issuer/subject. Developers only read/list/mutate their own records. An operator may inspect all owners' workloads/versions but may mutate only their own; inaccessible objects return 404. Tokens last five minutes; login/reload after expiry. Role/revocation changes can take effect only when existing tokens expire, because per-request introspection is not implemented. No placeholder/static-user header or fake login endpoint is supported.

Pages default to 20, maximum 100. Workloads are newest first; versions use descending version number. `next_cursor: null` means no more visible results. Treat cursors as opaque, URL-encode them, keep the same caller/collection/parent and use them within 15 minutes. Each page still requires a valid current token. A cursor from another owner/collection or one that is expired/tampered returns 400. Pages do not form a transaction snapshot.

Registration POSTs have no idempotency guarantee: retrying a version POST can create another version. After a timeout/connection loss, inspect the appropriate collection before deciding whether to retry. Duplicate workload names for an owner return 409. A stale ETag returns 412; reread and reconcile the intended metadata change before retrying PATCH. Future run idempotency is a separate contract and does not apply to these registration operations.

## Errors and current boundaries

Errors use `{"error":{"code":"...","message":"..."},"request_id":"..."}`. Responses include a server-generated `X-Request-ID`; keep that value for investigation, without sharing credentials. Payloads must be UTF-8 JSON, at most 64 KiB, with no duplicate keys or unknown fields.

| Status | Meaning/action |
|---|---|
| 400 | Malformed JSON, query, cursor or precondition header; correct the input. |
| 401 | Missing/invalid/expired access token; fresh login and reload. |
| 403 | Valid token without an allowed `forge-api` client role. |
| 404 | Missing/inaccessible resource or an unimplemented route. |
| 409 | Owner-scoped workload name conflict; inspect existing records. |
| 412 / 428 | Stale / missing `If-Match`; GET and reconcile against a fresh ETag. |
| 413 / 415 | Body over 64 KiB / unsupported content type. |
| 422 | Unsupported schema, workflow/model/tools or unapproved corpus snapshot/document IDs. |
| 503 | Dependency unavailable or operation deadline expired; check readiness and retained state before retrying ambiguous writes. |

The default corpus policy approves synthetic IDs only; it does not install a corpus, compute hashes or prove citations. There is no research execution, MCP tool process, Temporal dispatcher, report/history endpoint, quota enforcement or provisioning yet. Local HTTP and Keycloak development mode are for the trusted operator's machine; production TLS/identity operations and backup recovery remain later work. See [registration conventions](registration-api.md), [contract](api-contract.md) and [native operations](local-stack.md).
