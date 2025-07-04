#!/bin/bash

set -x

sts="redis-node"

echo "Wait for new sts to be present"

while true
do
    sts=$(kubectl -n "${INSTANCE_NAMESPACE}" get sts redis-node -o jsonpath='{.metadata.name}')
    if [ "$sts" = "redis-node" ]; then
        break;
    fi
done

replicas=$(kubectl -n "${INSTANCE_NAMESPACE}" get sts "${sts}" -o jsonpath='{.spec.replicas}')
echo "scale down sts"
kubectl -n "${INSTANCE_NAMESPACE}" scale sts "${sts}" --replicas=0

echo "wait for migration job to complete"

while true
do 
  job_status=$(kubectl -n "${INSTANCE_NAMESPACE}" get jobs "${MIGRATION_JOB}" -o jsonpath='{.status.succeeded}')
  if [ "$job_status" = "1" ]; then
    break;
  fi
done

echo "scale up sts"
kubectl -n "${INSTANCE_NAMESPACE}" scale sts "${sts}" --replicas="${replicas}"
