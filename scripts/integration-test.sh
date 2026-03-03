#!/usr/bin/env bash
# Integration test for probable-eureka CNI plugin.
# Exercises ADD → CHECK → DEL using CNI env-var invocation directly.
# Requires root; skip gracefully when not root.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ── root guard ───────────────────────────────────────────────────────────────
[ "$(id -u)" -eq 0 ] || { echo "skip: need root"; exit 0; }

# ── dependencies ─────────────────────────────────────────────────────────────
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required but not found"; exit 1; }

# ── constants ─────────────────────────────────────────────────────────────────
BINARY_NAME="probable-eureka"
CNI_BIN_DIR="/opt/cni/bin"
CNI_CONF_DIR="/etc/cni/net.d"
CONF_FILE="${CNI_CONF_DIR}/99-eureka-test.conflist"
BRIDGE_NAME="eureka0"
TEST_NS="eureka-test-ns"
TEST_NS_PATH="/var/run/netns/${TEST_NS}"
CONTAINER_ID="eureka-test-00000001"
DATA_DIR="$(mktemp -d /tmp/eureka-test-XXXXXX)"
CNI_CONFIG=""

# ── cleanup ───────────────────────────────────────────────────────────────────
cleanup() {
    local rc=$?
    set +e
    echo "--- cleanup ---"
    # Best-effort DEL in case the test failed before its own DEL
    if [ -n "${CNI_CONFIG}" ]; then
        CNI_COMMAND=DEL \
        CNI_CONTAINERID="${CONTAINER_ID}" \
        CNI_NETNS="${TEST_NS_PATH}" \
        CNI_IFNAME=eth0 \
        CNI_PATH="${CNI_BIN_DIR}" \
        "${CNI_BIN_DIR}/${BINARY_NAME}" <<< "${CNI_CONFIG}" 2>/dev/null || true
    fi
    ip netns del "${TEST_NS}" 2>/dev/null || true
    ip link del "${BRIDGE_NAME}" 2>/dev/null || true
    rm -f "${CONF_FILE}"
    rm -rf "${DATA_DIR}"
    exit "${rc}"
}
trap cleanup EXIT

# ── BUILD ─────────────────────────────────────────────────────────────────────
echo "=== BUILD ==="
cd "${REPO_DIR}"
make build

# ── INSTALL ───────────────────────────────────────────────────────────────────
echo "=== INSTALL ==="
mkdir -p "${CNI_BIN_DIR}"
install -m 755 "${BINARY_NAME}" "${CNI_BIN_DIR}/${BINARY_NAME}"

# ── CONFIG ────────────────────────────────────────────────────────────────────
echo "=== CONFIG ==="
mkdir -p "${CNI_CONF_DIR}"

CNI_CONFIG=$(cat <<EOCFG
{
  "cniVersion": "1.0.0",
  "name": "eureka-test",
  "type": "probable-eureka",
  "bridge": "${BRIDGE_NAME}",
  "ipam": {
    "type": "eureka-ipam",
    "dataDir": "${DATA_DIR}",
    "ranges": [
      [
        {
          "subnet": "10.88.0.0/24",
          "rangeStart": "10.88.0.10",
          "rangeEnd": "10.88.0.50",
          "gateway": "10.88.0.1"
        }
      ]
    ]
  }
}
EOCFG
)

# Write conflist (plugins array wrapping the single plugin config)
cat > "${CONF_FILE}" <<EOCL
{
  "cniVersion": "1.0.0",
  "name": "eureka-test",
  "plugins": [${CNI_CONFIG}]
}
EOCL

# ── NAMESPACE ─────────────────────────────────────────────────────────────────
echo "=== NAMESPACE ==="
ip netns add "${TEST_NS}"

# ── ADD ───────────────────────────────────────────────────────────────────────
echo "=== ADD ==="
ADD_RESULT=$(
    CNI_COMMAND=ADD \
    CNI_CONTAINERID="${CONTAINER_ID}" \
    CNI_NETNS="${TEST_NS_PATH}" \
    CNI_IFNAME=eth0 \
    CNI_PATH="${CNI_BIN_DIR}" \
    "${CNI_BIN_DIR}/${BINARY_NAME}" <<< "${CNI_CONFIG}"
)
echo "ADD result: ${ADD_RESULT}"

ALLOCATED_IP=$(echo "${ADD_RESULT}" | jq -r '.ips[0].address // empty')
[ -n "${ALLOCATED_IP}" ] || { echo "FAIL: ADD result missing IP"; exit 1; }
echo "ADD OK: allocated ${ALLOCATED_IP}"

# ── CHECK ─────────────────────────────────────────────────────────────────────
echo "=== CHECK ==="
CHECK_CONFIG=$(echo "${CNI_CONFIG}" | jq --argjson prev "${ADD_RESULT}" '. + {prevResult: $prev}')
CNI_COMMAND=CHECK \
CNI_CONTAINERID="${CONTAINER_ID}" \
CNI_NETNS="${TEST_NS_PATH}" \
CNI_IFNAME=eth0 \
CNI_PATH="${CNI_BIN_DIR}" \
"${CNI_BIN_DIR}/${BINARY_NAME}" <<< "${CHECK_CONFIG}"
echo "CHECK OK"

# ── DEL ───────────────────────────────────────────────────────────────────────
echo "=== DEL ==="
CNI_COMMAND=DEL \
CNI_CONTAINERID="${CONTAINER_ID}" \
CNI_NETNS="${TEST_NS_PATH}" \
CNI_IFNAME=eth0 \
CNI_PATH="${CNI_BIN_DIR}" \
"${CNI_BIN_DIR}/${BINARY_NAME}" <<< "${CNI_CONFIG}"
echo "DEL OK"

# Clear so cleanup skips the best-effort DEL (already done above)
CNI_CONFIG=""

echo ""
echo "=== INTEGRATION TEST PASSED ==="
