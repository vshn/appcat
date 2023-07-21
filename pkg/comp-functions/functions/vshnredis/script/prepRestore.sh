#!/bin/bash

set -euo pipefail

source_namespace=$(kubectl -n "${CLAIM_NAMESPACE}" get vshnredis "${CLAIM_NAME}" -ojson | jq -r '.status.instanceNamespace')

echo "copy secret"

access_key=$(kubectl -n "${source_namespace}" get secret backup-bucket-credentials -o template='{{ .data.AWS_ACCESS_KEY_ID | base64decode }}')
secret_key=$(kubectl -n "${source_namespace}" get secret backup-bucket-credentials -o template='{{ .data.AWS_SECRET_ACCESS_KEY | base64decode }}')
restic_password=$(kubectl -n "${source_namespace}" get secret k8up-repository-password -o template='{{ .data.password | base64decode }}')
restic_repository=$(kubectl -n "${source_namespace}" get snapshots.k8up.io "${BACKUP_NAME}" -o jsonpath='{.spec.repository}')
backup_path=$(kubectl -n "${source_namespace}" get snapshots.k8up.io "${BACKUP_NAME}" -o jsonpath='{.spec.paths[0]}')
backup_name=$(kubectl -n "${source_namespace}" get snapshots.k8up.io "${BACKUP_NAME}" -o jsonpath='{.spec.id}')
num_replicas=$(kubectl -n "${TARGET_NAMESPACE}" get statefulset redis-master -o jsonpath='{.spec.replicas}')
kubectl -n "${TARGET_NAMESPACE}" create secret generic "restore-credentials-${BACKUP_NAME}" --from-literal AWS_ACCESS_KEY_ID="${access_key}" --from-literal AWS_SECRET_ACCESS_KEY="${secret_key}" --from-literal RESTIC_PASSWORD="${restic_password}" --from-literal RESTIC_REPOSITORY="${restic_repository}" --from-literal BACKUP_PATH="${backup_path}" --from-literal BACKUP_NAME="${backup_name}"
kubectl create secret generic "statefulset-replicas-${CLAIM_NAME}-${BACKUP_NAME}" --from-literal NUM_REPLICAS="${num_replicas}"
echo "scaling down redis"

until kubectl -n "${TARGET_NAMESPACE}" get statefulset redis-master > /dev/null 2>&1
do
  sleep 1
done

kubectl -n "${TARGET_NAMESPACE}" scale statefulset redis-master --replicas 0 
