# Identity — F33

User tokens were already verified with Keycloak. This page separates that token from a service identity and a workload identity. [D48](decision-log.md#d48--f33-oidc-and-workload-identity) records the boundary. The verifier is still [internal/auth/oidc.go](../internal/auth/oidc.go). It was not replaced.

## User token

A user token is a Keycloak access token. The API checks the issuer, the `forge-api` audience, the RS256 signature, expiry, not-before, and a lifetime of at most five minutes. `typ` must be `Bearer`. The role is `developer` or `operator` under `resource_access.forge-api.roles`. `identity_kind` is absent or `user`. These tokens are the only ones that become a product principal.

Forged signatures, the wrong audience, and expired tokens are rejected. `TestAccessTokensAndKeyRotation` covers those cases.

## Service identity

`identity_kind` `service` with role `service`, and without `developer` or `operator`, is a service identity. The signature and expiry checks are the same. Product routes return 403. A service token is not a person and is not stored as a workload owner.

## Workload identity

A workload identity is the run id, workflow id `forge-run/<run UUID>`, tenant, project, and owner subject copied from the product run. It is not a bearer token. A token that claims `identity_kind` `workload` is rejected. `WorkloadFromRun` builds the struct and refuses a workflow id that does not match the run.

## Key rotation

The verifier discovers the issuer JWKS and caches the signing keys. `TestAccessTokensAndKeyRotation` still accepts a previously verified token while the JWKS endpoint is unavailable, then rotates the fixture key and accepts a token signed by the new key. An unknown signer is rejected. This demo does not define a production rotation period or a revocation list. Access tokens remain valid until their five-minute expiry.
