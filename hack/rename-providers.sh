#!/opt/homebrew/bin/bash

set -e

declare -A providerMap

providerMap["minio.crossplane.io"]="provider-minio"
providerMap["helm.crossplane.io"]="provider-helm"
providerMap["kubernetes.crossplane.io"]="provider-kubernetes"
providerMap["exoscale.crossplane.io"]="provider-exoscale"
providerMap["cloudscale.crossplane.io"]="provider-cloudscale"

kubectl --as cluster-admin apply -f hack/providers.yaml

# loop over each crd for each group
for c in "${!providerMap[@]}"; do
  for crd in $(kubectl --as cluster-admin get crd --ignore-not-found | grep "$c" | cut -d " " -f 1);
  do
    provider="${providerMap[$c]}"

    uid=$(kubectl --as cluster-admin get providers.pkg.crossplane.io "${provider}" --ignore-not-found -o=jsonpath='{.metadata.uid}' )

    jsonPatch=$(cat <<EOF
[
  {
    "op": "add",
    "path": "/metadata/ownerReferences/-",
    "value": {
      "apiVersion": "pkg.crossplane.io/v1",
      "blockOwnerDeletion": true,
      "controller": false,
      "kind": "Provider",
      "name": "$provider",
      "uid": "$uid"
    }
  }
]
EOF
    )

    kubectl --as cluster-admin patch crd "${crd}" --type=json -p="${jsonPatch}"

  done
done
