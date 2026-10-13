# Gateway, DNS, and TLS — F42

[D57](decision-log.md#d57--f42-gateway-dns-and-tls) records the boundary. The manifests are in [envoy.yaml](../deploy/gateway/envoy.yaml). They were not applied.

## Local path

The running API is loopback HTTP at `http://127.0.0.1:8081`. In kind, an unprivileged NGINX sidecar in the API pod forwards that loopback listener. That is the traffic path that exists. Envoy is a reviewed configuration for a later gateway, not a process that was started here.

## Routing and limits

The Envoy configuration routes `/api/`, `/healthz`, and `/readyz` to `api.forge-local.svc.cluster.local:8081`. A local rate limit allows 200 tokens per second on that listener. The API process already enforces its own per-tenant window. The Envoy limit was not observed, because Envoy was not started.

The Service is `ClusterIP`. It is not a LoadBalancer, a NodePort, or an Ingress. No cloud load balancer was created.

## DNS and TLS

No domain was bought. No public DNS name was delegated. No certificate was issued, and nothing renews one. The manifest contains no certificate and no private key.

A later TLS listener would terminate at Envoy using a Kubernetes Secret named `forge-gateway-tls`, created by the operator and kept out of Git. Issuance and renewal are not configured. This session did not verify TLS, because there is no certificate to present.

## Authentication

The gateway does not replace the API's bearer check. A request that reaches the API without a token is still rejected. The authenticated registration test on the disposable database returned 401 for a missing token and 201 for a developer token. That was HTTP on the test server, not TLS through Envoy.
