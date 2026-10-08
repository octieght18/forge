#!/bin/bash
# Install verified Kubernetes clients. Docker Engine must already be installed.
set -euo pipefail
test "$(id -u)" = 0
test "$(uname -m)" = x86_64
command -v docker >/dev/null
task_tmp=$(mktemp -d)
trap 'rm -f "$task_tmp/kind" "$task_tmp/kubectl"; rmdir "$task_tmp"' EXIT
curl --fail --location --silent --show-error https://github.com/kubernetes-sigs/kind/releases/download/v0.33.0/kind-linux-amd64 -o "$task_tmp/kind"
printf 'aee6151561422756b764a4ae28e7f44cda5af5a9eead3cc9985112b1de8d8e0d  %s\n' "$task_tmp/kind" | sha256sum --check
curl --fail --location --silent --show-error https://dl.k8s.io/release/v1.37.0/bin/linux/amd64/kubectl -o "$task_tmp/kubectl"
printf '6129359f4e1f3848a5572ccb0b26cf28b8ca08cef38c95a765b2f64a2c961a2f  %s\n' "$task_tmp/kubectl" | sha256sum --check
install -m 0755 "$task_tmp/kind" /usr/local/bin/kind
install -m 0755 "$task_tmp/kubectl" /usr/local/bin/kubectl
kind version
kubectl version --client
