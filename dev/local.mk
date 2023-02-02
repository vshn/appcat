.PHONY: local-install
local-install: export KUBECONFIG = $(KIND_KUBECONFIG)
local-install: export IMG_TAG=v0.0.1
local-install: export CONTAINER_IMAGE = ghcr.io/vshn/appcat-apiserver:${IMG_TAG}
local-install: kind-load-image   ## Install API service in local cluster
	kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/cluster/crds/apiextensions.crossplane.io_compositions.yaml
	kubectl apply -f config/namespace.yaml
	yq e '.spec.template.spec.containers[0].image=strenv(CONTAINER_IMAGE)' config/aggregated-apiserver.yaml | kubectl apply -f -
	kubectl apply -f 'config/api*'
	kubectl apply -f config/rbac.yaml
	kubectl apply -f dev/compositions

