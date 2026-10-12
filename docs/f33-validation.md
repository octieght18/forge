# F33 verification — 12 October 2026

F33 keeps Keycloak verification and separates user, service, and workload identity. [D48](decision-log.md#d48--f33-oidc-and-workload-identity) records the boundary. The description is [identity.md](identity.md).

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/auth
```

`TestAccessTokensAndKeyRotation` still rejects a forged signer, the wrong audience, and an expired token, and it still accepts a token after the fixture key rotates. A service token classifies as `service` and is not a user principal. A token claiming to be a workload is rejected. `WorkloadFromRun` accepts a matching `forge-run/<uuid>` and rejects a different workflow id. Empty tenant and project become the demo identifiers from F32.

No live Keycloak process was contacted. The test issuer is the signed fixture. Production revocation before expiry is not implemented.
