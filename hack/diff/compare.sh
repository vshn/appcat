#!/bin/bash

set -e

[ -z "${KUBECONFIG}" ] && echo "Please export KUBECONFIG" && exit 1

# get the state and all objects from each composite
function get_state() {
  type="$1"
  name="$2"
  dir_name="hack/tmp/$type-$name"

  echo "getting state of $type/$name"

  mkdir -p "$dir_name"

  while read -r res_type res_name
  do
    kubectl get "$res_type" "$res_name" -oyaml > "$dir_name/$res_type-$res_name.yaml"
  done <<< "$(kubectl get "$type" "$name" -oyaml | yq -r '.spec.resourceRefs | .[] | .kind + " " + .name')"

}

# also get the claim namespace
function get_claim_namespace() {
  type="$1"
  name="$2"
  dir_name="hack/tmp/$type-$name"

  ns=$(kubectl get "$type" "$name" -oyaml | yq -r '.metadata.labels["crossplane.io/claim-namespace"]')
  kubectl get ns "$ns" -oyaml > "$dir_name/namespace.yaml"

}

function run_func() {
  docker run -d --name func -p 9443:9443 ghcr.io/vshn/appcat:"$1" functions --insecure
  sleep 3
}

function stop_func() {
  docker rm -f func
}

function run_single_diff() {
  type="$1"
  name="$2"
  dir_name="hack/tmp/$type-$name"
  res_dir_name="hack/res/$type/$name"

  mkdir -p "$res_dir_name"

  kubectl get "$type" "$name" -oyaml > hack/tmp/xr.yaml
  comp=$(kubectl get "$type" "$name" -oyaml | yq -r '.spec.compositionRef.name')
  echo "composition: $comp $type/$name"
  kubectl get compositions.apiextensions.crossplane.io "$comp" -oyaml > hack/tmp/composition.yaml
  go run github.com/crossplane/crossplane/cmd/crank@v1.17.0 render hack/tmp/xr.yaml hack/tmp/composition.yaml hack/diff/function.yaml -o "$dir_name" > "$res_dir_name/$3.yaml"
}

function get_running_func_version() {
  version=$(kubectl get function function-appcat -oyaml | yq -r '.spec.package' | cut -d ":" -f2)
  echo "${version%"-func"}"
}

function get_pnt_func_version() {
  kubectl get function function-patch-and-transform -oyaml | yq -r '.spec.package' | cut -d ":" -f2
}

function template_func_file() {
  export PNT_VERSION=$1
  export APPCAT_VERSION=$2
  cat "$(dirname "$0")/function.yaml.tmpl" | envsubst > "$(dirname "$0")/function.yaml"
}

function diff_func() {
  # run_func "$1"
  # trap stop_func EXIT

  while read -r type name rest
  do
    # we only get the state on the first run for two reasons:
    # speed things up
    # avoid any diffs that could come from actual changes on the cluster
    [ "first" == "$2" ] && get_state "$type" "$name"
    get_claim_namespace "$type" "$name"
    run_single_diff "$type" "$name" "$2"
  done <<< "$(kubectl get composite --no-headers | sed 's/\// /g' )"

  # stop_func
}

# do the diff
function first_diff() {
  diff_func "$(get_running_func_version)" "first"
}

function second_diff() {
  diff_func "$(git rev-parse --abbrev-ref HEAD | sed 's/\//_/g')" "second"
}


function compare() {
  for f in hack/res/*/*;
  do
    # ignore changed array order
    # exclude nested managedFields in kube objects
    # exclude nested resourceVersion
    # don't print the huge dyff header
    go run github.com/homeport/dyff/cmd/dyff@v1.9.0 between \
    -i \
    --exclude-regexp "spec.forProvider.manifest.metadata.managedFields.*" \
    --exclude "spec.forProvider.manifest.metadata.resourceVersion" \
    --omit-header \
    "$f/first.yaml" "$f/second.yaml"
    # diff "$f/first.yaml" "$f/second.yaml"
  done
}

function clean() {
  rm -rf hack/tmp
  rm -rf hack/res
  rm -rf "$(dirname "$0")/function.yaml"
}

# stop_func
clean
trap clean EXIT

template_func_file "$(get_pnt_func_version)" "$(get_running_func_version)"

echo "Render live manifests"
first_diff

echo "Render against branch"
second_diff

echo "Comparing"
compare
