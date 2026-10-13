# Canary promotion — F44

[D59](decision-log.md#d59--f44-canary-promotion) records the boundary. The strategy is a canary. The command copies one release pin from the local overlay to the staging overlay.

## Procedure

The local pin is `deploy/gitops/overlays/local/release.txt`. Staging receives that same text only when the confirmation is exact:

```sh
go run ./cmd/forge-promote --overlays deploy/gitops/overlays --from local --to staging --confirm promote-local-to-staging
```

`--to cloud`, `--to eks`, and `--to gke` are refused. The selected cloud environment was not created. Promotion to that environment did not run.

## Success criteria

A canary is acceptable when the observation window has at least one request and the error rate is at or below 1 percent. An unauthenticated call to `/api/v1/workloads` remains 401. This session did not run that window against a live staging cluster. The 1 percent ceiling is the procedure's criterion, not a production SLO.

## Compatibility and rollback

Migrations are forward-only and are applied by `forge-migrate`, not by this command. Pointing Git back at the previous pin does not reverse a migration. A completed model call cannot be unsent. Temporal history is not rewritten. Retained execution rows are not deleted. There is no execution-status column to roll back.
