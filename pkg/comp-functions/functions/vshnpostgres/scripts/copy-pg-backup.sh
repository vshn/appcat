#!/bin/sh

set -e

xrdname=$(kubectl -n "${CLAIM_NAMESPACE}" get vshnpostgresqls "${CLAIM_NAME}" -ojson | jq -r '.spec.resourceRef.name')

source_namespace=$(kubectl -n "${CLAIM_NAMESPACE}" get vshnpostgresqls "${CLAIM_NAME}" -ojson | jq -r '.status.instanceNamespace')

echo "copy secret"
kubectl -n "${source_namespace}" get secret "pgbucket-${xrdname}" -ojson | jq 'del(.metadata.namespace) | del(.metadata.ownerReferences)' | kubectl -n "${TARGET_NAMESPACE}" apply -f -
echo "copy sgObjectStorage"
kubectl -n "${source_namespace}" get sgobjectstorages.stackgres.io "sgbackup-${xrdname}" -ojson | jq 'del(.metadata.namespace) | del(.metadata.ownerReferences)' | kubectl -n "$TARGET_NAMESPACE" apply -f -
echo "copy sgBackup"
kubectl -n "${source_namespace}" get sgbackups.stackgres.io "${BACKUP_NAME}" -ojson | jq '.spec.sgCluster = .metadata.namespace + "." + .spec.sgCluster | del(.metadata.namespace) | del(.metadata.ownerReferences)' | kubectl -n "${TARGET_NAMESPACE}" apply -f -
