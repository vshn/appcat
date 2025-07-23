#!/bin/bash
echo "Check if instance needs to be migrated"

timeout=300
elapsed=0
# The role and rolebinding might be created after the job has started and getting the PVCs will fail here.
# so we wait until we can successfully list the PVCs in the namespace before continuing.
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


echo "wait for sts to finish scaling down"
kubectl wait --for='jsonpath={.status.replicas}'=0 sts "${sts}" --timeout=300s

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

echo "Wait for pvc migration script to finish"
kubectl wait --for=condition=complete --timeout=300s -n "${INSTANCE_NAMESPACE}" job/"${INSTANCE_NAME}-pvc-migrationjob"

echo "scale up sts"
kubectl -n "${INSTANCE_NAMESPACE}" scale sts "${sts}" --replicas="${replicas}"
