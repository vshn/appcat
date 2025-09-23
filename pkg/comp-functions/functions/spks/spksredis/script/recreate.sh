#!/bin/bash

set -eox pipefail

name="$STS_NAME"
namespace="$STS_NAMESPACE"
size="$STS_SIZE"
release="$RELEASE_NAME"

control_secret_namespace="spks-crossplane"
control_secret_name="control-plane-kubeconfig"

control_kubeconfig="$(mktemp)"
cleanup() {
  if [[ -n "$release" ]]; then
    "${kubectl[@]}" annotate release "$release" "crossplane.io/paused-" || true
  fi
  rm -f "${control_kubeconfig}";
}
trap cleanup EXIT

use_control_kubeconfig=0
if kubectl -n "${control_secret_namespace}" get secret "${control_secret_name}" -o go-template='{{index .data "config" | base64decode}}' > "${control_kubeconfig}" 2>/dev/null && [[ -s "${control_kubeconfig}" ]]; then
  echo "Using control-plane kubeconfig from ${control_secret_namespace}/${control_secret_name}"
  use_control_kubeconfig=1
else
  echo "Falling back to default context (secret missing/forbidden/empty)"
fi

if [[ "${use_control_kubeconfig}" -eq 1 ]]; then
  kubectl=(kubectl --kubeconfig "${control_kubeconfig}")
else
  kubectl=(kubectl)
fi

echo "Checking if the PVC sizes match"
# Check if delete is necessary
found=$(kubectl -n "$namespace" get sts "$name" -o json --ignore-not-found)

foundsize=$(echo -En "$found" | jq -r '.spec.volumeClaimTemplates[] | select(.metadata.name=="data") | .spec.resources.requests.storage')
paused=$("${kubectl[@]}" get release "$release" -o jsonpath='{.metadata.annotations.crossplane\.io\/paused}' --ignore-not-found)

if [[ "${foundsize}" != "${size}" && "${paused}" == "true" ]]; then
  echo "PVC sizes don't match, deleting sts"
  # We try to delete the sts and wait for 5s. On APPUiO it can happen that the
  # deletion with orphan doesn't go through and the sts is stuck with an orphan finalizer.
  # So if the delete hasn't returned after 5s we forcefully patch away the finalizer.
  kubectl -n "${namespace}" delete sts "${name}" --cascade=orphan --ignore-not-found --wait=true --timeout 5s || true
  kubectl -n "${namespace}" patch sts "${name}" -p '{"metadata":{"finalizers":null}}' || true
  # Upause the release so that the sts is recreated. We pause the release to avoid provider-helm updating the release
  # before the sts is deleted.
  echo "Triggering sts re-creation"
  "${kubectl[@]}" annotate release "$release" "crossplane.io/paused-"
  "${kubectl[@]}" patch release "${release}" --type merge -p "{\"spec\":{\"forProvider\":{\"values\":{\"persistentVolume\":{\"size\":\"foo\"}}}}}"
  "${kubectl[@]}" patch release "${release}" --type merge -p "{\"spec\":{\"forProvider\":{\"values\":{\"persistentVolume\":{\"size\":\"${size}\"}}}}}"

  count=0
  while ! kubectl -n "$namespace" get sts "$name" && [[ count -lt 300 ]]; do
    echo "waiting for sts to re-appear"
    count=$((count + 1))
    sleep 1
  done
  [[ $count -lt 300 ]] || (echo "Waited for 5 minutes for sts to re-appear"; exit 1)

  echo "Set label on sts to trigger the statefulset-resize-controller"
  kubectl -n "${namespace}" label sts "${name}" --overwrite "sts-resize.vshn.net/resize-inplace=true"
else
  echo "Sizes match, nothing to do"
fi