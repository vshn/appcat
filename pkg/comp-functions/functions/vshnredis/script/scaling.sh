#!/bin/bash

sts="redis-node"

echo "Wait for new sts to be present"

while true
do
    sts=$(kubectl -n "${INSTANCE_NAMESPACE}" get sts redis-node -o jsonpath='{.metadata.name}')
    if [ "$sts" = "redis-node" ]; then
        break;
    fi
    sleep 1
done

replicas=$(kubectl -n "${INSTANCE_NAMESPACE}" get sts "${sts}" -o jsonpath='{.spec.replicas}')
echo "scale down sts"
kubectl -n "${INSTANCE_NAMESPACE}" scale sts "${sts}" --replicas=0

echo "Create migration job"

kubectl apply -f - <<EOF
  apiVersion: batch/v1
  kind: Job
  metadata:
    name: "pvc-migrationjob"
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


while true
do 
  job_status=$(kubectl -n "${INSTANCE_NAMESPACE}" get jobs pvc-migrationjob -o jsonpath='{.status.succeeded}')
  if [ "$job_status" = "1" ]; then
    break;
  fi
  sleep 1
done

echo "scale up sts"
kubectl -n "${INSTANCE_NAMESPACE}" scale sts "${sts}" --replicas="${replicas}"
