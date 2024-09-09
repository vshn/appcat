#!/bin/bash

# get the state and all objects from each composite
function get_state() {
  type="$1"
  name="$2"
  dir_name="tmp/$type-$name"

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
  dir_name="tmp/$type-$name"

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
  dir_name="tmp/$type-$name"
  res_dir_name="res/$type/$name"

  mkdir -p "$res_dir_name"

  kubectl get "$type" "$name" -oyaml > xr.yaml
  comp=$(kubectl get "$type" "$name" -oyaml | yq -r '.spec.compositionRef.name')
  echo "composition: $comp"
  kubectl get compositions.apiextensions.crossplane.io "$comp" -oyaml > composition.yaml
  go run github.com/crossplane/crossplane/cmd/crank@v1.17.0 render hack/diff/xr.yaml hack/diff/composition.yaml hack/diff/function.yaml -o "$dir_name" > "$res_dir_name/$3.yaml"
}

function get_running_func_version() {
  version=$(kubectl get function function-appcat -oyaml | yq -r '.spec.package' | cut -d ":" -f2)
  echo "${version%"-func"}"
}

function diff() {
  run_func "$1"
  trap stop_func EXIT

  while read -r type name rest
  do
    get_state "$type" "$name"
    get_claim_namespace "$type" "$name"
    run_single_diff "$type" "$name" "$2"
  done <<< "$(kubectl get composite --no-headers | sed 's/\// /g' )"
}

# do the diff
function first_diff() {
  diff "$(get_running_func_version)" "first"
}

function second_diff() {
  diff "$(git rev-parse --abbrev-ref HEAD | sed 's/\//_/g')" "second"
}

stop_func
rm -rf tmp
rm -rf res

first_diff
seconf_diff
