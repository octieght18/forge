# F12a/F13 validation — 8 October 2026

F12a restores the Kubernetes CRD/controller prerequisite alongside completed F12 command reconciliation. F13 implements one namespace per workload, fixed resource/runtime policy and safe asynchronous cleanup under [D27](decision-log.md#d27--restored-environment-controller-and-workload-boundaries). [The environment guide](execution-environments.md) explains commands, authority and limitations.

## Reproducible checks

Go tests exercise denied/unavailable identity lookup and recovery, duplicate stable reconciliation without status rewrites, quota/token-default drift, partial child creation, foreign resource/namespace conflicts, storage-blocked deletion and foreign finalizer retention. A concurrent ownership-change regression verifies optimistic resourceVersion fencing blocks stale repair. Real PostgreSQL tests install the shipped column grants on a disposable role/database, verify issuer/subject matching and deny product writes, sensitive reads and schema creation.

Python guards verify CRD immutability/status separation and the controller's restricted runtime/Secret/RBAC boundaries. Existing Windows/Linux formatting, module integrity, vet, build, race and product checks continue. The native CI job also deploys the real controller into kind after a retained database copy.

The real-cluster smoke has two phases:

```bash
python3 deploy/kubernetes/environment_smoke.py seed
# Stop and restart the retained kind stack through the normal operator commands.
python3 deploy/kubernetes/environment_smoke.py verify
```

`seed` uses two real Keycloak authorization-code/PKCE logins and adds two synthetic workload records, then:

- Rejects a forged owner before namespace creation; deletes that failed intent through normal finalization.
- Repeats accepted intent while checking unchanged environment/namespace UIDs.
- Rejects invalid profiles, owner mutation, unknown deployment fields and mismatched CR names through real API-server validation.
- Stops the controller, deletes two children, alters quota and submits the second owner's intent; restarting restores missing children, repairs drift and creates the second boundary.
- Verifies worker accounts have no implicit token mount and cannot read shared Secrets or create Pods.
- Exercises real Restricted admission, per-container maxima, default request/limit mutation, absence of projected service-account tokens and the two-Pod quota.

`verify` checks exact environment and namespace UIDs after a whole-node restart, then creates a synthetic static CSI PV reference with no disk/Pod to prove cleanup refuses persistent storage. After the test removes only that synthetic PV, a test-owned ConfigMap finalizer holds namespace cleanup; the controller preserves it and retains its own environment finalizer. The test finalizer's owner releases its hold, normal cleanup completes, and `forge-local` plus the retained PostgreSQL PV remain.

Review corrected the negative schema probes to use strict server-side dry-run replacement: `kubectl patch` does not support `--validate`. The corrected probes assert actual validation reasons and passed profile, immutable-owner, unknown-field and name rejections against the live API server. This supersedes the earlier negative-probe result.

## Local observations

Real-kind `seed` passed all owner, replay, partial-creation, drift, schema, admission and worker-RBAC checks on this PC. Whole-node restart preserved exact environment/namespace UIDs. `verify` passed persistent-storage refusal, foreign-finalizer retention, normal cleanup and shared-database protection. The Windows operator wrapper separately passed apply, JSON status, Ready observation and delete for an existing synthetic fixture without adding another product record.

Full Linux real-PostgreSQL race testing passed **181 test/subtest events across ten tested packages**, including the new read-only role and concurrent-ownership tests. Vet/build, eight native guards, six Kubernetes guards, documentation and PowerShell parsing passed. Windows Go tests and build passed with a permitted private Temp directory. An initial sandbox run denied token-file replacement in its restricted Temp directory; rerunning outside that Temp sandbox resolved it without login-code changes. A WSL connection attempt timed out during concurrent dependency compilation; the services remained healthy, and verification resumed when WSL became responsive. No distro reset or data replacement was used.

Hosted CI runs the same real environment seed/restart/verify phases on the published commit, along with Windows/Linux and PostgreSQL jobs. Card completion requires all four jobs to pass for that exact commit. The CI link is recorded on the completed F12a/F13 cards.

Private proof contains the synthetic identities and retained UIDs in `~/.local/share/forge-kubernetes/f13-proof.json`; generated credentials, tokens, kubeconfig and raw diagnostics stay local. The two synthetic product workload records remain after environment cleanup because product deletion is unsupported. Native original databases are retained.

## Material limits

Fake-client tests do not reproduce admission, garbage collection or namespace finalization; real-kind smoke covers those separately. No network-isolation, worker-execution, public environment endpoint, load/performance target or compromised-controller isolation claim is made. Existing native Balanced thresholds remain specific to the native measurements. This restores infrastructure scope with a privileged controller and additional Kubernetes client dependencies; later network/worker/API designs remain separate decisions.
