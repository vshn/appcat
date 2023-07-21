#!/bin/bash

set -euo pipefail

echo "Wait for restore to complete"

counter=0
until [ $counter -eq 300 ] || [[ $(kubectl -n "${TARGET_NAMESPACE}" get job "${RESTORE_JOB_NAME}" -o jsonpath='{.status.succeeded}' 2> /dev/null) -eq 1 ]];
do
  (( counter+=1 ))
  sleep 1
done

echo "scaling up redis"

kubectl -n "${TARGET_NAMESPACE}" scale statefulset redis-master --replicas "${NUM_REPLICAS}"

echo "cleanup secret"

kubectl -n "${TARGET_NAMESPACE}" delete secret "restore-credentials-${BACKUP_NAME}"
kubectl delete secret "statefulset-replicas-${CLAIM_NAME}-${BACKUP_NAME}"
