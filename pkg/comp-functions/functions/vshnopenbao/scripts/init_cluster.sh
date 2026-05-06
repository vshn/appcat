#!/bin/bash
set -euo pipefail

echo "Waiting for OpenBao to become reachable..."
until curl -sk "${VAULT_ADDR}/v1/sys/health" > /dev/null 2>&1; do
  echo "  not ready, retrying in 5s..."
  sleep 5
done

INIT_STATUS=$(curl -sk "${VAULT_ADDR}/v1/sys/init" | jq -r '.initialized')
if [ "${INIT_STATUS}" = "true" ]; then
  echo "Already initialized, nothing to do."
  exit 0
fi

echo "Initializing OpenBao..."
INIT_RESPONSE=$(curl -sk --request POST \
  --data "{\"secret_shares\": ${SECRET_SHARES}, \"secret_threshold\": ${SECRET_THRESHOLD}}" \
  "${VAULT_ADDR}/v1/sys/init")

ROOT_TOKEN=$(echo "${INIT_RESPONSE}" | jq -r '.root_token')
if [ -z "${ROOT_TOKEN}" ] || [ "${ROOT_TOKEN}" = "null" ]; then
  echo "ERROR: no root_token in response"
  exit 1
fi

KEYS_JSON=$(echo "${INIT_RESPONSE}" | jq 'del(.root_token)')

# kubectl create secret fails with "already exists" if the secret was already created (e.g. job retries, pod restarts).
# The pipe pattern combines both: `create --dry-run=client -o yaml` generates the manifest locally without hitting
# the API, then `kubectl apply -f` - creates or updates it safely.
#
# The `--save-config` adds the last-applied-configuration annotation so future kubectl apply calls can do a proper
# three-way merge diff.
echo "Storing init output to secret ${ROOT_TOKEN_SECRET_NAME}..."
kubectl -n "${NAMESPACE}" create secret generic "${ROOT_TOKEN_SECRET_NAME}" \
  --from-literal=VAULT_ADDR="${VAULT_ADDR}" \
  --from-literal=VAULT_TOKEN="${ROOT_TOKEN}" \
  --save-config --dry-run=client -o yaml | kubectl apply -f -

echo "Storing unseal/recovery keys to secret ${UNSEAL_KEYS_SECRET_NAME}..."
kubectl -n "${NAMESPACE}" create secret generic "${UNSEAL_KEYS_SECRET_NAME}" \
  --from-literal=keys="${KEYS_JSON}" \
  --save-config --dry-run=client -o yaml | kubectl apply -f -
