# Local Kubernetes in WSL

The owner accepted containerizing the current API, PostgreSQL and Keycloak in a single-node **kind** cluster inside Ubuntu WSL, copying both existing databases, preserving the public loopback URLs and retaining native originals (D26). This supersedes D20's no-container deployment choice. D27 subsequently adds the separate [environment CRD/controller](execution-environments.md) and per-workload resource/runtime boundaries; executable research workers remain later work.

The API image contains static Go API, migration and environment-controller binaries, runs as UID 65532, and has a scratch runtime with CA certificates. Its build context allows only cmd/, internal/, module files and Dockerfile. Git history, raw F10 profiles, private configuration and database backups cannot enter the context. Dependencies are locked by digest in [images.json](../deploy/kubernetes/images.json); no image is published to a registry.

## Prerequisites and startup

Use x86-64 Ubuntu WSL with systemd, Docker Engine 29.8.2 and buildx, and the existing [native installation](local-stack.md). Docker was installed from its [official Ubuntu repository](https://docs.docker.com/engine/install/ubuntu/), without adding the user to the privileged Docker group. On another PC, install Docker following that guide, then install verified kind 0.33.0/kubectl 1.37.0 with `sudo bash deploy/kubernetes/install-tools.sh`. The installer verifies pinned SHA-256 checksums. The Kubernetes node image is 1.37.0 with its matching kind release digest.

From the repository in Windows PowerShell, copy the native installation **once**:

```powershell
./scripts/kubernetes-stack.ps1 import-native
```

The command stops native API/identity writes, takes private logical backups of both databases, starts PostgreSQL in kind, creates the separate database roles and restores both databases with their ownership/grants. It validates and preserves the native UTF-8/C.UTF-8/libc database locale. Before Keycloak runs, every table's row count and sorted row digest must equal the source, with UTC timestamps and C ordering for a portable comparison. Existing passwords, OIDC realm, signing keys, client configuration, user subjects, cursor key and corpus policy are copied. Native files are retained. Import refuses an already initialized target database; failed partial restores require local operator investigation using the retained backups, never automatic overwrite. This demo's bounded restore accepts at most 256 MiB per dump; larger data needs a separately reviewed procedure.

Normal lifecycle commands:

```powershell
./scripts/kubernetes-stack.ps1 up
./scripts/kubernetes-stack.ps1 status
./scripts/kubernetes-stack.ps1 login
./scripts/kubernetes-stack.ps1 stop
```

`up` builds a content-tagged local API image, exports amd64 image archives, loads kind, runs an explicit bounded migration Job, reapplies runtime/dispatcher grants, and waits for PostgreSQL, Keycloak and API readiness. It does not start a dispatcher. Startup and stop are serialized by a private lock. The archive preserves a tag as well as platform selection; this avoids Docker's containerd image-store import errors without changing host-wide storage settings. See [kind's known issues](https://kind.sigs.k8s.io/docs/user/known-issues/#unable-to-kind-load-docker-images).

`stop` drains/removes application Pods, stops PostgreSQL gracefully, stops forwarding and stops the kind node container. It retains the cluster and all data. `up` restarts the same node. No purge or cluster-delete command is provided. A hidden unprivileged Windows-launched hold process keeps WSL running while forwards are active.

## Access and boundaries

The migration Job has a bounded authenticated database readiness initializer so a node restart's networking delay cannot race migration startup. The NGINX proxy is a native Kubernetes sidecar: its own startup probe must succeed before API initialization, and it stops after API draining. NGINX streams request bodies; the API retains its 64 KiB bound and JSON error format. All NGINX temporary directories are on its writable /tmp volume.

API: `http://127.0.0.1:8081`. Keycloak: `http://127.0.0.1:8082`. Issuer remains `http://127.0.0.1:8082/realms/forge`. The browser login callback still needs a running forge-login helper on port 8083. Login uses the existing native helper binary; credentials remain in the copied private secrets.json. API examples accept the resulting private token file just as before.

The API still binds only Pod loopback, retaining F03's configuration guard. An unprivileged NGINX sidecar has a Pod-facing 8080 listener forwarding to that API, and a loopback-only 8082 listener forwarding the fixed Forge realm path to the Keycloak Service. This lets exact-origin discovery/JWKS checks keep working inside the Pod. No caller chooses the proxy destination; no general forwarding proxy is exposed. Services are ClusterIP; systemd-managed kubectl forwards bind only host 127.0.0.1. The Kubernetes API also binds host loopback on 6445. There is no Ingress, NodePort or LoadBalancer.

All application containers are non-root, drop capabilities, disallow privilege escalation and use RuntimeDefault seccomp. The namespace enforces the restricted Pod security standard. Containers have explicit requests/limits and no mounted service account token. API/NGINX roots are read-only. The stock local-development Keycloak image needs a writable root for its startup augmentation; it runs start-dev as the same version as the source database. These local HTTP/dev defaults are not a production deployment. The migration Job receives only migration credentials; API receives only runtime database credentials and its cursor key. Dispatcher credentials are retained privately but not mounted into any live process.

## Data, backups and rollback

Private WSL state: `/home/owner/.local/share/forge-kubernetes/` (selected user's home if changed). It contains secrets.json, cursor.key, corpus-policy.json, a private administrator kubeconfig, private logical backups, import proof and tokens. The PostgreSQL files are under `data/postgres/18/docker/`, bind-mounted into the kind node and exposed through a Retain PersistentVolume/PVC. They survive Pod replacement and normal node stop/start. Native originals remain under `/home/owner/.local/share/forge-native/`.

The kind cluster is a local trusted development boundary: its administrator can read Kubernetes Secrets, and default etcd secret storage is not encrypted. Host filesystem privacy and kubeconfig permissions matter. The 20 GiB PV capacity is a declaration, not a host filesystem quota. The default kind network does not demonstrate enforced NetworkPolicy or multi-tenant isolation. There is no HA, cloud deployment, registry release, SBOM/signing/scanning, automatic backup schedule or tested disaster recovery yet.

After cutover, native and Kubernetes databases are independent. Do not run both stacks on the shared ports or alternate between them expecting synchronization. To inspect the old installation, stop Kubernetes first and use native startup; the originals represent the migration snapshot and omit later Kubernetes changes. Keep current private backups before any rollback involving new data. Full cluster deletion/recreation and restoration are operator work, not normal stop. Do not delete the retained PV, PVC, kind cluster or state directory casually.

Research versions remain declarations of the fixed workflow. Temporal, its worker image, MCP process and model integration join when those tickets are implemented. There is no arbitrary uploaded code, image-per-workload provisioning or CRD controller here. The original F13 environment reconciliation, F15 templates, F39 supply-chain checks and F43 GitOps requirements stay separate future work. Native Balanced performance thresholds are not transferred to Kubernetes; a new comparable baseline needs review.

See [deployment resources](../deploy/kubernetes/resources.py), [lifecycle command](../deploy/kubernetes/stack.py) and [validation](kubernetes-validation.md).
