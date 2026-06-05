#!/bin/sh
set -eu

# Only pod-0 initializes the cluster.
case "${POD_NAME}" in
  *-0) echo "Running on pod-0, proceeding with init check." ;;
  *)   echo "Not pod-0 (${POD_NAME}), skipping."; exec sleep infinity ;;
esac

K8S_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
K8S_CACERT=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
K8S_API="https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}/api/v1"

# Idempotency check: if root-token secret already exists, credentials were already saved
# (handles pod restarts after a successful init).
echo "Checking if credentials secret already exists..."
HTTP_STATUS=$(SSL_CERT_FILE="${K8S_CACERT}" wget -qS --spider \
  --header="Authorization: Bearer ${K8S_TOKEN}" \
  "${K8S_API}/namespaces/${NAMESPACE}/secrets/${ROOT_TOKEN_SECRET_NAME}" 2>&1 \
  | awk '/HTTP/{print $2}' | head -1) || true
if [ "${HTTP_STATUS}" = "200" ]; then
  echo "Credentials already saved (secret ${ROOT_TOKEN_SECRET_NAME} exists), nothing to do."
  exec sleep infinity
fi

# Wait for OpenBao to be reachable and determine initialization state.
# bao operator init -status outputs JSON with "initialized": true/false when the server is up.
# On connection errors it outputs an error string (no JSON) — we detect by absence of the key.
echo "Waiting for OpenBao to become reachable at ${VAULT_INIT_ADDR}..."
while true; do
  STATUS=$(bao operator init -status \
    -address="${VAULT_INIT_ADDR}" \
    -ca-cert="/tls/ca.crt" \
    -format=json 2>&1) || true

  if echo "${STATUS}" | grep -qi '"initialized".*true'; then
    echo "ERROR: OpenBao is already initialized but the credentials secret is missing."
    echo "Manual recovery required — the init output cannot be retrieved after the fact."
    exec sleep infinity
  fi

  if echo "${STATUS}" | grep -qi '"initialized".*false'; then
    echo "OpenBao is up and not yet initialized. Proceeding."
    break
  fi

  echo "OpenBao not yet reachable, retrying in 5s... (response: ${STATUS})"
  sleep 5
done

# Initialize OpenBao with Shamir's secret sharing.
echo "Initializing OpenBao (shares=${SECRET_SHARES}, threshold=${SECRET_THRESHOLD})..."
bao operator init \
  -address="${VAULT_INIT_ADDR}" \
  -ca-cert="/tls/ca.crt" \
  -key-shares="${SECRET_SHARES}" \
  -key-threshold="${SECRET_THRESHOLD}" \
  -format=json > /tmp/init-output.json

ROOT_TOKEN=$(awk -F'"' '/"root_token"/{print $4}' /tmp/init-output.json)
if [ -z "${ROOT_TOKEN}" ]; then

  echo "ERROR: root_token missing from init output:"
  cat /tmp/init-output.json
  exit 1
fi
echo "Initialization successful. Storing credentials to Kubernetes secrets..."

# Create the root-token secret via the Kubernetes API.
# Using stringData avoids the need for base64 encoding.
printf '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"%s","namespace":"%s"},"stringData":{"VAULT_ADDR":"%s","VAULT_TOKEN":"%s"}}\n' \
  "${ROOT_TOKEN_SECRET_NAME}" "${NAMESPACE}" "${VAULT_ADDR}" "${ROOT_TOKEN}" \
  > /tmp/root-token-request.json
SSL_CERT_FILE="${K8S_CACERT}" wget -q -O /dev/null \
  --header="Authorization: Bearer ${K8S_TOKEN}" \
  --header="Content-Type: application/json" \
  --post-file=/tmp/root-token-request.json \
  "${K8S_API}/namespaces/${NAMESPACE}/secrets"
echo "Root-token secret created: ${ROOT_TOKEN_SECRET_NAME}"

# Create the unseal-keys secret with the full init output.
# The init JSON is escaped for embedding as a JSON string value:
# backslashes doubled, double-quotes escaped, newlines replaced with literal \n sequence.
KEYS_ESCAPED=$(awk '{gsub(/\\/, "\\\\"); gsub(/"/, "\\\""); printf "%s\\n", $0}' /tmp/init-output.json \
  | sed 's/\\n$//')
printf '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"%s","namespace":"%s"},"stringData":{"keys":"%s"}}\n' \
  "${UNSEAL_KEYS_SECRET_NAME}" "${NAMESPACE}" "${KEYS_ESCAPED}" \
  > /tmp/unseal-keys-request.json
SSL_CERT_FILE="${K8S_CACERT}" wget -q -O /dev/null \
  --header="Authorization: Bearer ${K8S_TOKEN}" \
  --header="Content-Type: application/json" \
  --post-file=/tmp/unseal-keys-request.json \
  "${K8S_API}/namespaces/${NAMESPACE}/secrets"
echo "Unseal-keys secret created: ${UNSEAL_KEYS_SECRET_NAME}"

echo "Initialization complete."
exec sleep infinity
