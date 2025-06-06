#!/bin/sh

set -xe

sourcexrdname=$(kubectl -n "${CLAIM_NAMESPACE}" get vshnkeycloak "${CLAIM_NAME}" -ojson | jq -r '.spec.resourceRef.name')
targetxrdname=$(kubectl -n "${CLAIM_NAMESPACE}" get vshnkeycloak "${TARGET_CLAIM}" -ojson | jq -r '.spec.resourceRef.name')


source_namespace=$(kubectl get xvshnkeycloak "${sourcexrdname}" -ojson | jq -r '.status.instanceNamespace')


echo "copy secret"
kubectl -n "${source_namespace}" get secret "${sourcexrdname}"-credentials-secret -ojson | jq --arg targetxrdname "${targetxrdname}" 'del(.metadata.namespace) | del(.metadata.labels) | del(.metadata.annotations) | del(.metadata.resourceVersion) | del(.metadata.uid) | .metadata.name = $targetxrdname + "-credentials-secret"' | kubectl -n "${TARGET_NAMESPACE}" apply -f -
