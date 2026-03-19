#!/bin/bash
set -euo pipefail

# Env vars: OPENBAO_ADDR, INSTANCE_NAMESPACE, INIT_SECRET_NAME, AUTO_UNSEAL

echo "Waiting for OpenBao to become reachable..."
until curl -sk "${OPENBAO_ADDR}/v1/sys/health" > /dev/null 2>&1; do
  echo "  not ready, retrying in 5s..."
  sleep 5
done

INIT_STATUS=$(curl -sk "${OPENBAO_ADDR}/v1/sys/init" | jq -r '.initialized')
if [ "${INIT_STATUS}" = "true" ]; then
  echo "Already initialized, nothing to do."
  exit 0
fi

echo "Initializing OpenBao..."
INIT_RESPONSE=$(curl -sk --request POST \
  --data '{"secret_shares": 5, "secret_threshold": 3}' \
  "${OPENBAO_ADDR}/v1/sys/init")

ROOT_TOKEN=$(echo "${INIT_RESPONSE}" | jq -r '.root_token')
if [ -z "${ROOT_TOKEN}" ] || [ "${ROOT_TOKEN}" = "null" ]; then
  echo "ERROR: no root_token in response"
  exit 1
fi

echo "Storing init output to secret ${INIT_SECRET_NAME}..."
kubectl -n "${INSTANCE_NAMESPACE}" create secret generic "${INIT_SECRET_NAME}" \
  --from-literal=root_token="${ROOT_TOKEN}" \
  --from-literal=unseal_key_0="$(echo "${INIT_RESPONSE}" | jq -r '.keys[0]')" \
  --from-literal=unseal_key_1="$(echo "${INIT_RESPONSE}" | jq -r '.keys[1]')" \
  --from-literal=unseal_key_2="$(echo "${INIT_RESPONSE}" | jq -r '.keys[2]')" \
  --from-literal=unseal_key_3="$(echo "${INIT_RESPONSE}" | jq -r '.keys[3]')" \
  --from-literal=unseal_key_4="$(echo "${INIT_RESPONSE}" | jq -r '.keys[4]')" \
  --from-literal=init_response="${INIT_RESPONSE}" \
  --save-config --dry-run=client -o yaml | kubectl apply -f -

if [ "${AUTO_UNSEAL}" = "true" ]; then
  echo "Auto-unseal enabled, skipping manual unseal."
  exit 0
fi

echo "Unsealing OpenBao..."
for KEY in \
  "$(echo "${INIT_RESPONSE}" | jq -r '.keys[0]')" \
  "$(echo "${INIT_RESPONSE}" | jq -r '.keys[1]')" \
  "$(echo "${INIT_RESPONSE}" | jq -r '.keys[2]')"; do
  RESP=$(curl -sk --request POST --data "{\"key\": \"${KEY}\"}" "${OPENBAO_ADDR}/v1/sys/unseal")
  SEALED=$(echo "${RESP}" | jq -r '.sealed')
  echo "  key applied, sealed=${SEALED}"
  if [ "${SEALED}" = "false" ]; then
    echo "OpenBao is unsealed."
    break
  fi
done
