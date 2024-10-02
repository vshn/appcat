#!/bin/bash

set -e

debug=$1

[ -z "${KUBECONFIG}" ] && echo "Please export KUBECONFIG" && exit 1

# get the state and all objects from each composite
function get_state() {
  type="$1"
  name="$2"
  dir_name="hack/tmp/$type-$name"

  echo "getting state of $type/$name"

  mkdir -p "$dir_name"

  while read -r res_type res_name api_version
  do
    group=$(echo "$api_version" | cut -d "/" -f1)
    kubectl get "$res_type"."$group" "$res_name" -oyaml > "$dir_name/$res_type-$res_name.yaml"
  done <<< "$(kubectl get "$type" "$name" -oyaml | yq -r '.spec.resourceRefs | .[] | .kind + " " + .name + " " + .apiVersion')"

}

# also get the claim namespace
function get_claim_namespace() {
  type="$1"
  name="$2"
  dir_name="hack/tmp/$type-$name"

  ns=$(kubectl get "$type" "$name" -oyaml | yq -r '.metadata.labels["crossplane.io/claim-namespace"]')
  kubectl get ns "$ns" -oyaml > "$dir_name/namespace.yaml"

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
  crank_func render hack/tmp/xr.yaml hack/tmp/composition.yaml hack/diff/function.yaml -o "$dir_name" > "$res_dir_name/$3.yaml"
}

function crank_func() {
  mkdir -p .work/bin
  [ ! -f .work/bin/crank ] && curl -s https://releases.crossplane.io/stable/v1.17.0/bin/linux_amd64/crank -o .work/bin/crank
  chmod +x .work/bin/crank
  if .work/bin/crank -h > /dev/null 2>&1; then .work/bin/crank "$@";
  else go run github.com/crossplane/crossplane/cmd/crank@v1.17.0 "$@"; fi
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
  export DEBUG=$3
  cat "$(dirname "$0")/function.yaml.tmpl" | envsubst > "$(dirname "$0")/function.yaml"
}

function diff_func() {

  while read -r type name rest
  do
    # we only get the state on the first run for two reasons:
    # speed things up
    # avoid any diffs that could come from actual changes on the cluster
    [ "first" == "$1" ] && get_state "$type" "$name"
    get_claim_namespace "$type" "$name"
    run_single_diff "$type" "$name" "$1"
  done <<< "$(kubectl get composite --no-headers | sed 's/\// /g' )"

}

# do the diff
function first_diff() {
  diff_func "first"
}

function second_diff() {
  diff_func "second"
}


function compare() {
  for f in hack/res/*/*;
  do
    echo "comparing $f"
    # enable color
    # ignore changed array order
    # exclude nested managedFields in kube objects
    # exclude nested resourceVersion
    # don't print the huge dyff header
    dyff between \
    --color=on \
    -i \
    --exclude-regexp "spec.forProvider.manifest.metadata.managedFields.*" \
    --exclude "spec.forProvider.manifest.metadata.resourceVersion" \
    --omit-header \
    "$f/first.yaml" "$f/second.yaml"
    # diff "$f/first.yaml" "$f/second.yaml"
  done
}

function dyff() {
  mkdir -p .work/bin
  [ ! -f .work/bin/dyff ] && curl -sL https://github.com/homeport/dyff/releases/download/v1.9.1/dyff_1.9.1_linux_amd64.tar.gz -o .work/bin/dyff.tar.gz && \
  tar xvfz .work/bin/dyff.tar.gz -C .work/bin > /dev/null 2>&1
  chmod +x .work/bin/dyff
  if .work/bin/dyff version > /dev/null 2>&1 ; then .work/bin/dyff "$@";
  else go run github.com/homeport/dyff/cmd/dyff@v1.9.1 "$@"; fi
}

function clean() {
  rm -rf hack/tmp
  rm -rf hack/res
  rm -rf "$(dirname "$0")/function.yaml"
}

clean
trap clean EXIT

template_func_file "$(get_pnt_func_version)" "$(get_running_func_version)" ""

echo "Render live manifests"
first_diff

template_func_file "$(get_pnt_func_version)" "$(git rev-parse --abbrev-ref HEAD | sed 's/\//_/g')" "$debug"

echo "Render against branch"
second_diff

echo "Comparing"
compare
