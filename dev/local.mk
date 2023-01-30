
.PHONY: local-install
local-install: export KUBECONFIG = $(KIND_KUBECONFIG)
local-install: export CONTAINER_IMAGE = $(IMG)
local-install: kind-load-image   ## Install Operator in local cluster
	yq e -i '.spec.template.spec.containers[0].image=strenv(CONTAINER_IMAGE)' dev/config/aggregated-apiserver.yaml
	kubectl apply -f dev/config
