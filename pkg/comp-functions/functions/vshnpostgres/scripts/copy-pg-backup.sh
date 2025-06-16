#!/bin/sh

set -xe

xrdtype="${CLAIM_TYPE}"

if [ "${xrdtype}" != "" ]
then
    xrdservicename=$(kubectl -n "${CLAIM_NAMESPACE}" get "${xrdtype#?}" "${CLAIM_NAME}" -ojson | jq -r '.spec.resourceRef.name')
    xrdname=$(kubectl -n "${CLAIM_NAMESPACE}" get "${xrdtype}" "${xrdservicename}" -ojson | jq -r '.spec.resourceRefs | map(select(.kind == "XVSHNPostgreSQL")) | .[].name')
else
    xrdname=$(kubectl -n "${CLAIM_NAMESPACE}" get vshnpostgresqls "${CLAIM_NAME}" -ojson | jq -r '.spec.resourceRef.name')
fi
source_namespace=$(kubectl get xvshnpostgresqls "${xrdname}" -ojson | jq -r '.status.instanceNamespace')

echo "copy secret"
kubectl -n "${source_namespace}" get secret "pgbucket-${xrdname}" -ojson | jq 'del(.metadata.namespace) | del(.metadata.ownerReferences)' | kubectl -n "${TARGET_NAMESPACE}" apply -f -
echo "copy sgObjectStorage"
kubectl -n "${source_namespace}" get sgobjectstorages.stackgres.io "sgbackup-${xrdname}" -ojson | jq 'del(.metadata.namespace) | del(.metadata.ownerReferences)' | kubectl -n "$TARGET_NAMESPACE" apply -f -
echo "copy sgBackup"
kubectl -n "${source_namespace}" get sgbackups.stackgres.io "${BACKUP_NAME}" -ojson | jq '.spec.sgCluster = .metadata.namespace + "." + .spec.sgCluster | del(.metadata.namespace) | del(.metadata.ownerReferences)' | kubectl -n "${TARGET_NAMESPACE}" apply -f -
