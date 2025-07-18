#!/bin/bash
echo "Check if instance needs to be migrated"

timeout=300
elapsed=0
while ! kubectl get pvc -n "$INSTANCE_NAMESPACE" >/dev/null 2>&1; 
do
  echo "missing permissions, waiting for rbac role to be present"
  if [ "${elapsed}" -ge "${timeout}" ]; then
    echo "Job timed out after ${timeout} seconds. Exiting."
    exit 1
  fi

  sleep 1
  elapsed=$((elapsed + 1))
done

pvc=$(kubectl get pvc -n "$INSTANCE_NAMESPACE" -o jsonpath='{.items[?(@.metadata.name=="redis-data-redis-master-0")].metadata.name}')
if [ -z "${pvc}" ]; then
  echo "Nothing to migrate. Exiting"
  exit 0
fi


echo "Wait for new sts to be present"

while true
do
    sts=$(kubectl -n "${INSTANCE_NAMESPACE}" get sts redis-node -o jsonpath='{.metadata.name}')
    if [ "${sts}" = "redis-node" ]; then
        break;
    fi
    sleep 1
done

replicas=$(kubectl -n "${INSTANCE_NAMESPACE}" get sts "${sts}" -o jsonpath='{.spec.replicas}')
echo "scale down sts"
kubectl -n "${INSTANCE_NAMESPACE}" scale sts "${sts}" --replicas=0


echo "wait for sts to finish scaling"
timeout=300
elapsed=0
while true
do
  stsStatus=$(kubectl -n "${INSTANCE_NAMESPACE}" get sts "${sts}" -o jsonpath='{.status.replicas}')
  if [ "$stsStatus" -eq 0 ]; then
    break;
  fi
  
  if [ "${elapsed}" -ge "${timeout}" ]; then
    echo "Job timed out after ${timeout} seconds. Exiting."
    exit 1
  fi

  sleep 1
  elapsed=$((elapsed + 1))
done

echo "Create migration job"

kubectl apply -f - <<EOF
  apiVersion: batch/v1
  kind: Job
  metadata:
    name: "${INSTANCE_NAME}-pvc-migrationjob"
    namespace: "${INSTANCE_NAMESPACE}"
  spec:
    template:
      spec:
        containers:
        - name: rsync
          image: instrumentisto/rsync-ssh
          command:
            - rsync
            - -avhWHAX
            - --no-compress
            - --progress
            - /src/dump.rdb
            - /dst/
          volumeMounts:
            - mountPath: "/src"
              name: "src"
            - mountPath: "/dst"
              name: "dst"
        restartPolicy: OnFailure
        volumes:
          - name: src
            persistentVolumeClaim:
              claimName: redis-data-redis-master-0
              readOnly: true
          - name: dst
            persistentVolumeClaim:
              claimName: redis-data-redis-node-0
EOF


timeout=300
elapsed=0
while true
do 
  job_status=$(kubectl -n "${INSTANCE_NAMESPACE}" get jobs "${INSTANCE_NAME}"-pvc-migrationjob -o jsonpath='{.status.succeeded}')
  job_failed=$(kubectl -n "${INSTANCE_NAMESPACE}" get jobs "${INSTANCE_NAME}"-pvc-migrationjob -o jsonpath='{.status.failed}')

  if [ "${job_status}" = "1" ]; then
    echo "Job completed successfully."
    break
  elif [ "${job_failed}" = "1" ]; then
    echo "Job failed. Exiting."
    exit 1
  fi

  if [ "${elapsed}" -ge "${timeout}" ]; then
    echo "Job timed out after ${timeout} seconds. Exiting."
    exit 1
  fi

  sleep 1
  elapsed=$((elapsed + 1))
done

echo "scale up sts"
kubectl -n "${INSTANCE_NAMESPACE}" scale sts "${sts}" --replicas="${replicas}"
