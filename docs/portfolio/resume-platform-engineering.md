# Resume variant — Platform Engineering / Developer Infrastructure

Evidence-backed bullets from the Forge repository only. No invented employer history.

## Summary

Platform-minded software engineer building a self-service control plane that registers AI workloads, provisions isolated environments, and exposes durable asynchronous execution on top of Kubernetes, Temporal, Keycloak, and PostgreSQL—without rebuilding those systems.

## Selected evidence bullets

- Designed and documented a Go HTTP/OpenAPI registration API with seven operations, immutable `research.v1` versions, ETag concurrency, and **405** on DELETE; clean-checkout seven-ops path verified with real Keycloak PKCE ([f09-validation.md](../f09-validation.md)).
- Measured native retained-data startup median **66.5 s** and authenticated registration journey median **22 ms** across repeated trials with raw JSONL evidence ([f08-baseline.md](../f08-baseline.md)).
- Delivered asynchronous environment provisioning (`202 accepted`) for three workload templates; provision-accept median **4.041 ms**; developers receive **no kubeconfig** ([f19-validation.md](../f19-validation.md), [g2-validation.md](../g2-validation.md)).
- Built canary promote/rollback tooling; injected regression restored staging pin in **54.421 µs** while retaining execution identifiers ([f45-validation.md](../f45-validation.md)).
- Ran disposable PostgreSQL backup/restore drill: dump **61.76 ms**, checked restore **71.79 ms**; documented Temporal/evidence as out of dump scope ([f46-validation.md](../f46-validation.md)).
- Published EKS Terraform module, Envoy/GitOps manifests, and cloud deployment guide with explicit **apply-not-run** and ~**$123/mo** illustrative floor ([f49-validation.md](../f49-validation.md), [cloud-deployment-guide.md](../cloud-deployment-guide.md)).
- Authored operator runbooks and three **simulated** incident reports tied to failure-injection samples ([runbooks/README.md](../runbooks/README.md)).

## Honest limits

G3–G6 open. No production SLO. Cloud resources not applied. Kind NetworkPolicy not enforced.
