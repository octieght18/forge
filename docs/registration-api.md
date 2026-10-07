# F06 — Authenticated workload registration

F06 implements the seven F04 workload/version operations with PostgreSQL, real access-token verification, owner checks, strong ETags and signed cursor pages. Runs, cancellation, reports and evidence remain later HTTP implementation work. Versions cannot be changed or deleted; workload metadata can be updated, with no ownership transfer or deletion.

The owner accepted this scope, Keycloak token/role conventions, 15-minute signed cursors, a five-second operation timeout, database readiness and operator-managed corpus approvals on 6 October 2026. Registration POSTs have **no idempotency guarantee**. Workload duplicate names conflict within an owner; retrying version POST may create another immutable version. F05's owner/key idempotency applies to future run submission/rerun endpoints. The original ticket's broader CRUD/idempotency wording is amended accordingly.

F07 deploys real native Keycloak and PostgreSQL, and F09 supplies the [developer quickstart](quickstart.md) and [current runnable API examples](api-examples.md). Use those for generated credentials, browser/PKCE login and the exact local issuer. The configuration below is for an operator managing the process independently.

## Activate the product API

The default process retains F03's health-only mode. Enable registration explicitly with complete private configuration; partial configuration fails startup rather than serving unauthenticated product endpoints. The database must already have F05 migrations and runtime grants. Startup verifies required tables/permissions and performs OIDC discovery; it never migrates. Use the runtime database role, not an administrator or migration role.

| Variable | Value |
| --- | --- |
| `FORGE_PRODUCT_API` | `true` to enable registration; unset/false retains health-only mode |
| `FORGE_DATABASE_URL` | Private runtime PostgreSQL connection; use a private password file or injected secret |
| `FORGE_OIDC_ISSUER` | Exact Keycloak realm issuer; HTTPS, or loopback HTTP for development |
| `FORGE_CURSOR_KEY_FILE` | Private file containing base64 for at least 32 random bytes |
| `FORGE_CORPUS_POLICY_FILE` | Operator's approved snapshot/document map |
| `FORGE_OPERATION_TIMEOUT` | Default `5s`, configurable positive duration up to `10s`, within HTTP read/write timeouts |

The audience/client-role namespace is `forge-api`. The loopback HTTP address and F03 transport/lifecycle settings still apply. Cursor and policy files are bounded; connection strings, tokens, file contents and identity claims are omitted from routine logs and client errors. Secure the private files using Windows filesystem permissions; keep them outside Git. Preserve the cursor key across process restarts; changing it invalidates outstanding cursors. No secret-generation endpoint exists.

PowerShell example with credentials supplied through your private password mechanism:

```powershell
$env:FORGE_PRODUCT_API = 'true'
$env:FORGE_DATABASE_URL = 'postgres://forge_runtime@127.0.0.1:5432/forge?sslmode=disable'
$env:FORGE_OIDC_ISSUER = 'http://127.0.0.1:8082/realms/forge'
$env:FORGE_CURSOR_KEY_FILE = 'C:\private\forge-cursor.key'
$env:FORGE_CORPUS_POLICY_FILE = 'C:\private\forge-corpus-policy.json'
go run ./cmd/api
```

Those paths/database credentials are examples, not provisioned resources. The actual native stack uses dedicated PostgreSQL port **55436**, generates private configuration and starts the issuer above. The [native guide](local-stack.md) owns deployment, persistent identity data and token-helper setup; F06's handler code itself does not install them. See [PostgreSQL setup](persistence.md) for independently managed databases.

## Keycloak access-token policy

Use a `forge-api` resource client with client roles `developer` and `operator`, and a separate public login client using authorization code with required S256 PKCE. Disable implicit/password grants and service accounts for the developer login client. Ensure access tokens contain audience `forge-api` and the configured client's roles at `resource_access.forge-api.roles`; unrelated realm/client roles confer no access. Configure a five-minute access-token lifespan and RS256 signing. Exact redirect URIs and client/realm import setup follow the deployment ticket. [Keycloak role/audience/PKCE configuration](https://www.keycloak.org/docs/latest/server_admin/index.html).

The pinned go-oidc verifier checks signature, exact issuer, audience and expiry; Forge additionally requires Keycloak's signed `typ: Bearer` claim, a subject, issued-at and a lifetime no longer than 300 seconds. Future issued/not-before values, ID tokens, bound tokens and unapproved roles are rejected. API servers and the issuer need synchronized clocks. Ownership uses signed issuer/subject, never email/display name, a body field or an identity header. Operator roles permit all-owner inspection, with mutations confined to their own issuer/subject.

Discovery and keys are fetched only from the operator-configured issuer and a same-origin JWKS endpoint, over trusted TLS or local development HTTP. Redirects are disabled and responses/time are bounded. JWT-supplied key URLs do not determine trust. Cached verified keys permit verification while the issuer is temporarily unavailable; an unverifiable token, including one needing unavailable keys, gets a generic 401. New key IDs trigger refresh. There is no per-request introspection: logout, role removal or key withdrawal can leave an already issued token usable until its expiry, at most five minutes. A cursor never extends that token lifetime. [Pinned go-oidc](https://pkg.go.dev/github.com/coreos/go-oidc/v3@v3.21.0/oidc).

## Requests, identities and concurrency

Operations and payloads remain defined by the [OpenAPI document](../internal/contract/openapi.json) and [API contract guide](api-contract.md). Successful writes return stable server-issued resource UUIDs and `Location`; every response has a fresh server-generated `X-Request-ID` and `Cache-Control: no-store`. The seven OpenAPI operation IDs are unchanged. Product methods reject unsupported operations rather than silently adding delete/transfer behavior.

Send `Authorization: Bearer <access_token>` for each product request. Missing/invalid tokens yield 401 with a Bearer challenge; valid tokens lacking a permitted client role yield 403. Missing or inaccessible objects both yield 404. Bodies must be UTF-8 `application/json`, at most 64 KiB, with no duplicate keys or unknown fields. Query parameters are validated strictly, including duplicate/unknown parameters and page limits.

Workload create/read/update returns a strong ETag of `"<workload UUID>:<revision>"`. PATCH requires the exact ETag from an authorized read: missing is 428, invalid is 400, stale is 412. Authorization precedes precondition disclosure; the database revision check prevents concurrent writers from overwriting each other. Version registration serializes numbers within the owned workload.

Lists default to 20 entries and accept at most 100. Workloads use descending creation time/UUID; versions use descending version number. The opaque URL-safe cursor encodes a position authenticated with HMAC-SHA256, binding issuer/subject/role, collection and parent/filter. It expires after 15 minutes. Tampered, expired or mismatched tokens return 400. Each page reauthenticates and filters ownership before limiting. Page size may change; there is no cross-page transaction snapshot guarantee. The response returns `next_cursor: null` when no further visible entry exists.

## Corpus approval and availability

The policy maps lowercase snapshot SHA-256 IDs to permitted document-ID arrays, with at most 64 snapshots and 64 unique documents per snapshot, inside 64 KiB. [The example policy](../deploy/corpus-policy.example.json) is synthetic; replace it with operator-approved IDs. A new version must use an approved snapshot and a subset of its documents. Unknown snapshots/documents or unsupported tools/model/workflow/spec fields return 422. Policy is loaded at startup; restart after an operator-approved update.

This policy approves IDs only. It does not prove files exist, compute their hashes, mount a corpus or validate citations. Later corpus/MCP activities must resolve the same immutable snapshot, check actual hashes and re-enforce permissions. Existing version reads remain possible if approval changes; future run acceptance/activities must apply current policy. No model call occurs during registration.

`/healthz` remains process liveness. Product-mode `/readyz` requires process readiness and accessible product tables. PostgreSQL failures and expired operation deadlines return safe 503 errors; cancellation reaches database queries and body reads. A database lock timeout does not mutate the workload. Transport/client disconnects can prevent delivery of any response, and ambiguous registration commits retain F04's retry limitations.

## Verification limits

Local and CI integration tests send signed RSA access tokens through real HTTP to actual PostgreSQL repositories under runtime credentials. They cover allowed/forbidden ownership, immutable versions, schema errors, strong preconditions, approved sources, cursor binding/tampering, concurrent updates, lock timeouts and database connection failure. JWT tests check invalid claims/signers, role mapping, cached keys and rotation. Those handler tests use a fixture OIDC discovery/JWKS server. Separate F07 native smoke/CI exercises deployed Keycloak login/PKCE and retained identity/product records; F09 also runs the published client example against the real token. See [F07 evidence](f07-validation.md) and [F09 verification](f09-validation.md).
