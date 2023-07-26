#!/bin/bash

set -euo pipefail

echo "Wait for restore to complete"

until [[ $(kubectl -n "${TARGET_NAMESPACE}" get job "${RESTORE_JOB_NAME}" -o jsonpath='{.status.succeeded}' 2> /dev/null) -eq 1 ]] || [[ $(kubectl -n "${TARGET_NAMESPACE}" get job "${RESTORE_JOB_NAME}" -o jsonpath='{.status.failed}' 2> /dev/null) -eq 1 ]];
do
  sleep 1
done

if [[ $(kubectl -n "${TARGET_NAMESPACE}" get job "${RESTORE_JOB_NAME}" -o jsonpath='{.status.failed}' 2> /dev/null) -eq 1 ]]; then
  kubectl apply -f - <<EOF
  apiVersion: v1
  kind: Event
  metadata:
    name: "${DEST_CLAIM_NAME}-restore-failed"
    namespace: "${CLAIM_NAMESPACE}"
  type: Warning
  firstTimestamp: $(date --utc +%FT%TZ)
  lastTimestamp: $(date --utc +%FT%TZ)
  message: "Restore of ${DEST_CLAIM_NAME} failed"
  involvedObject:
    apiVersion: vshn.appcat.vshn.io/v1
    kind: VSHNRedis
    name: "${DEST_CLAIM_NAME}"
    namespace: "${CLAIM_NAMESPACE}"
    uid: "$(kubectl -n ${CLAIM_NAMESPACE} get vshnredis.vshn.appcat.vshn.io ${DEST_CLAIM_NAME} -o jsonpath='{.metadata.uid}')"
  reason: RestoreFailed
  source:
    component: "${TARGET_NAMESPACE}/${RESTORE_JOB_NAME}/job.batch/v1"
EOF
fi

if [[ $(kubectl -n "${TARGET_NAMESPACE}" get job "${RESTORE_JOB_NAME}" -o jsonpath='{.status.succeeded}' 2> /dev/null) -eq 1 ]]; then
  kubectl apply -f - <<EOF
  apiVersion: v1
  kind: Event
  metadata:
    name: "${DEST_CLAIM_NAME}-restore-completed"
    namespace: "${CLAIM_NAMESPACE}"
  type: Normal
  firstTimestamp: $(date --utc +%FT%TZ)
  lastTimestamp: $(date --utc +%FT%TZ)
  message: "Restore of ${DEST_CLAIM_NAME} succeeded"
  involvedObject:
    apiVersion: vshn.appcat.vshn.io/v1
    kind: VSHNRedis
    name: "${DEST_CLAIM_NAME}"
    namespace: "${CLAIM_NAMESPACE}"
    uid: "$(kubectl -n ${CLAIM_NAMESPACE} get vshnredis.vshn.appcat.vshn.io ${DEST_CLAIM_NAME} -o jsonpath='{.metadata.uid}')"
  reason: RestoreSucceeded
  source:
    component: "${TARGET_NAMESPACE}/${RESTORE_JOB_NAME}/job.batch/v1"
EOF
fi
echo "scaling up redis"

kubectl -n "${TARGET_NAMESPACE}" scale statefulset redis-master --replicas "${NUM_REPLICAS}"

echo "cleanup secret"

kubectl -n "${TARGET_NAMESPACE}" delete secret "restore-credentials-${BACKUP_NAME}"
kubectl delete secret "statefulset-replicas-${SOURCE_CLAIM_NAME}-${BACKUP_NAME}"
