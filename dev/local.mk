crossplane_sentinel = $(kind_dir)/crossplane_sentinel
stackgres_sentinel = $(kind_dir)/stackgres_sentinel
cert_manager_sentinel = $(kind_dir)/cert_manager_sentinel

.PHONY: local-install ## Install dependencies, AppCat APIServer and apply test cases
local-install: export KUBECONFIG = $(KIND_KUBECONFIG)
local-install: local-debug appcat-apiserver apply-test-cases

.PHONY: local-debug ## Install dependencies and apply test cases. API Server should be installed separately for debugging purposes
local-debug: export KUBECONFIG = $(KIND_KUBECONFIG)
local-debug: export IMG_TAG=v0.0.1
local-debug: kind-load-image install-dependencies

.PHONY: apply-test-cases ## Apply test cases to the running cluster
local-debug: export KUBECONFIG = $(KIND_KUBECONFIG)
apply-test-cases:
	kubectl apply -f dev/compositions
	kubectl apply -f dev/backup
	make .wait-provider provider=helm
	make .wait-provider provider=kubernetes
	kubectl apply -f dev/backup/provider-configs
	kubectl apply -f dev/backup/claims

.PHONY: install-dependencies ## Installs all dependencies for this API Server
install-dependencies: export KUBECONFIG = $(KIND_KUBECONFIG)
install-dependencies: crossplane stackgres cert-manager

.PHONY: crossplane
crossplane: $(crossplane_sentinel) ## Installs Crossplane in kind cluster

$(crossplane_sentinel): export KUBECONFIG = $(KIND_KUBECONFIG)
$(crossplane_sentinel): $(KIND_KUBECONFIG)
	helm repo add --force-update crossplane https://charts.crossplane.io/stable
	helm repo update
	helm upgrade --install crossplane crossplane/crossplane \
		--create-namespace \
		--namespace syn-crossplane \
		--set "args[0]='--debug'" \
		--set "args[1]='--enable-composition-revisions'" \
		--set "args[2]='--enable-composition-functions'" \
		--set "args[3]='--enable-environment-configs'" \
		--set "xfn.enabled=true" \
		--set "xfn.image.repository=ghcr.io/zugao/appcat-comp-functions" \
		--set "xfn.image.tag=latest" \
		--set webhooks.enabled=true \
		--wait
	@touch $@

.PHONY: stackgres
stackgres: $(stackgres_sentinel) ## Installs Stackgres in kind cluster and create a cluster with backup for testing purposes

$(stackgres_sentinel): export KUBECONFIG = $(KIND_KUBECONFIG)
$(stackgres_sentinel): $(KIND_KUBECONFIG)
	helm repo add --force-update stackgres-charts https://stackgres.io/downloads/stackgres-k8s/stackgres/helm/
	helm repo update
	helm upgrade --install stackgres-operator stackgres-charts/stackgres-operator \
		--create-namespace \
		--namespace stackgres-system \
		--wait
	@touch $@

.PHONY: cert-manager
cert-manager: $(cert_manager_sentinel) ## Installs Cert Manager in kind cluster

$(cert_manager_sentinel): export KUBECONFIG = $(KIND_KUBECONFIG)
$(cert_manager_sentinel): $(KIND_KUBECONFIG)
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
	@touch $@

.PHONY: appcat-apiserver ## Installs AppCat APIServer in kind cluster
appcat-apiserver: export KUBECONFIG = $(KIND_KUBECONFIG)
appcat-apiserver: export IMG_TAG=v0.0.1
appcat-apiserver: export CONTAINER_IMAGE = ghcr.io/vshn/appcat-apiserver:${IMG_TAG}
appcat-apiserver: install-dependencies
	kubectl apply -f config/namespace.yaml
	kubectl apply -f config/
	yq e '.spec.template.spec.containers[0].image=strenv(CONTAINER_IMAGE)' config/aggregated-apiserver.yaml | kubectl apply -f -
	yq e '.spec.insecureSkipTLSVerify=true' config/apiservice.yaml | kubectl apply -f -

.PHONY: .wait-provider
.wait-provider:
	@until kubectl get pod -n syn-crossplane --selector=pkg.crossplane.io/provider=provider-$(provider) -o=jsonpath='{.items[0].metadata.name}' >/dev/null 2>&1; do \
	  echo 'Waiting for providers'; \
	  sleep 1; \
	done
	kubectl wait --timeout=60s --for=condition=Ready pod --selector=pkg.crossplane.io/provider=provider-$(provider) -n syn-crossplane

