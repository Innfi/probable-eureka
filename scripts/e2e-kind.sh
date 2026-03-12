#!/usr/bin/env bash
# e2e-kind.sh — end-to-end CNI test on a local kind cluster.
# Requirements: kind, kubectl, docker (or podman), make.
set -euo pipefail

CLUSTER_NAME="eureka-e2e"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ---------------------------------------------------------------------------
# cleanup — always runs on EXIT
# ---------------------------------------------------------------------------
cleanup() {
    echo "==> Cleaning up: deleting kind cluster ${CLUSTER_NAME}"
    kind delete cluster --name "${CLUSTER_NAME}" 2>/dev/null || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# 1. Prerequisite check
# ---------------------------------------------------------------------------
echo "==> Checking prerequisites"
missing=0
for cmd in kind kubectl make; do
    if ! command -v "${cmd}" &>/dev/null; then
        echo "ERROR: required command not found: ${cmd}" >&2
        missing=1
    fi
done

if command -v docker &>/dev/null; then
    CONTAINER_CLI=docker
elif command -v podman &>/dev/null; then
    CONTAINER_CLI=podman
else
    echo "ERROR: neither docker nor podman found" >&2
    missing=1
    CONTAINER_CLI=docker  # placeholder; won't be reached
fi

[ "${missing}" -eq 0 ] || exit 1
echo "  OK (container runtime: ${CONTAINER_CLI})"

# ---------------------------------------------------------------------------
# 2. Build CNI installer image
# ---------------------------------------------------------------------------
echo "==> Building CNI installer image (make image)"
cd "${REPO_ROOT}"
make image

# ---------------------------------------------------------------------------
# 3. Create kind cluster (delete pre-existing one with the same name first)
# ---------------------------------------------------------------------------
echo "==> Creating kind cluster: ${CLUSTER_NAME}"
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo "  Pre-existing cluster found — deleting it first"
    kind delete cluster --name "${CLUSTER_NAME}"
fi
kind create cluster --name "${CLUSTER_NAME}" \
    --config "${SCRIPT_DIR}/kind-config.yaml"

# ---------------------------------------------------------------------------
# 4. Load installer image into the cluster
# ---------------------------------------------------------------------------
echo "==> Loading probable-eureka:latest into kind cluster"
kind load docker-image probable-eureka:latest --name "${CLUSTER_NAME}"

# ---------------------------------------------------------------------------
# 5. Install CNI (ConfigMap + DaemonSet)
# ---------------------------------------------------------------------------
echo "==> Applying CNI manifests"
kubectl apply \
    -f "${REPO_ROOT}/deployments/configmap.yaml" \
    -f "${REPO_ROOT}/deployments/daemonset.yaml"

# ---------------------------------------------------------------------------
# 6. Wait for DaemonSet rollout
# ---------------------------------------------------------------------------
echo "==> Waiting for eureka-cni-installer rollout (timeout 180s)"
kubectl rollout status daemonset/eureka-cni-installer \
    -n kube-system --timeout=180s

# ---------------------------------------------------------------------------
# 7. Deploy test workload: nginx Deployment (2 replicas) + ClusterIP Service
# ---------------------------------------------------------------------------
echo "==> Deploying nginx test workload"
kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx-test
  template:
    metadata:
      labels:
        app: nginx-test
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-test
  namespace: default
spec:
  selector:
    app: nginx-test
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
EOF

# ---------------------------------------------------------------------------
# 8. Wait for both pods to be Running/Ready
# ---------------------------------------------------------------------------
echo "==> Waiting for nginx pods to be Ready (timeout 180s)"
kubectl wait --for=condition=Ready pod \
    -l app=nginx-test -n default --timeout=180s

# ---------------------------------------------------------------------------
# 9. Configure gateway IP on the cni0 bridge inside the kind node
#    (the bridge is created by the CNI on first pod ADD; set gateway here)
# ---------------------------------------------------------------------------
echo "==> Configuring 10.244.0.1/16 on cni0 inside the kind node"
"${CONTAINER_CLI}" exec "${CLUSTER_NAME}-control-plane" \
    ip addr add 10.244.0.1/16 dev cni0 2>/dev/null || true

# ---------------------------------------------------------------------------
# 10. Connectivity checks
# ---------------------------------------------------------------------------
echo "==> Running connectivity checks"

POD1=$(kubectl get pod -l app=nginx-test -n default \
    -o jsonpath='{.items[0].metadata.name}')
POD2=$(kubectl get pod -l app=nginx-test -n default \
    -o jsonpath='{.items[1].metadata.name}')
POD2_IP=$(kubectl get pod "${POD2}" -n default \
    -o jsonpath='{.status.podIP}')
SVC_IP=$(kubectl get svc nginx-test -n default \
    -o jsonpath='{.spec.clusterIP}')

echo "  pod1=${POD1}  pod2=${POD2} (${POD2_IP})  svc=${SVC_IP}"

# (a) pod-to-pod
echo "==> Check (a): pod-to-pod (${POD1} -> ${POD2_IP}:80)"
kubectl exec "${POD1}" -n default -- \
    wget -qO /dev/null --timeout=10 "http://${POD2_IP}"
echo "  PASS"

# (b) pod-to-service
echo "==> Check (b): pod-to-service (${POD1} -> ${SVC_IP}:80)"
kubectl exec "${POD1}" -n default -- \
    wget -qO /dev/null --timeout=10 "http://${SVC_IP}"
echo "  PASS"

# (c) egress
echo "==> Check (c): egress (${POD1} -> http://example.com)"
kubectl exec "${POD1}" -n default -- \
    wget -qO /dev/null --timeout=10 "http://example.com"
echo "  PASS"

echo ""
echo "==> All connectivity checks passed!"
