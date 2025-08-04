#!/bin/bash

set -eox pipefail

name="$STS_NAME"
namespace="$STS_NAMESPACE"
size="$STS_SIZE"
release="$COMPOSITION_NAME"

echo "Checking if the PVC sizes match"
# Check if delete is necessary
found=$(kubectl -n "$namespace" get sts "$name" -o json --ignore-not-found)

foundsize=$(echo -En "$found" | jq -r '.spec.volumeClaimTemplates[] | select(.metadata.name=="redis-data") | .spec.resources.requests.storage')

if [[ $foundsize != "$size" ]]; then
  echo "PVC sizes don't match, deleting sts"
  # We try to delete the sts and wait for 5s. On APPUiO it can happen that the
  # deletion with orphan doesn't go through and the sts is stuck with an orphan finalizer.
  # So if the delete hasn't returned after 5s we forcefully patch away the finalizer.
  kubectl -n "$namespace" delete sts "$name" --cascade=orphan --ignore-not-found --wait=true --timeout 5s || true
  kubectl -n "$namespace" patch sts "$name" -p '{"metadata":{"finalizers":null}}' || true
  # Poke the release so it tries again to create the sts
  # We first set it to garbage to ensure that the release is in an invalid state, we use an invalid state so it doesn't
  # actually deploy anything.
  # Then we patch the right size to enforce an upgrade
  # This is necessary as provider-helm doesn't actually retry failed helm deployments unless the values change.
  echo "Triggering sts re-creation"
  kubectl patch release "$release" --type merge -p "{\"spec\":{\"forProvider\":{\"values\":{\"replica\":{\"persistence\":{\"size\":\"foo\"}}}}}}"
  kubectl patch release "$release" --type merge -p "{\"spec\":{\"forProvider\":{\"values\":{\"replica\":{\"persistence\":{\"size\":\"$size\"}}}}}}"
  count=0
  while ! kubectl -n "$namespace" get sts "$name" && [[ count -lt 300 ]]; do
    echo "waiting for sts to re-appear"
    count=$count+1
    sleep 1
  done
  [[ $count -lt 300 ]] || (echo "Waited for 5 minutes for sts to re-appear"; exit 1)
  echo "Set label on sts to trigger the statefulset-resize-controller"
  kubectl -n "$namespace" label sts "$name" --overwrite "sts-resize.vshn.net/resize-inplace=true"
else
  echo "Sizes match, nothing to do"
fi
