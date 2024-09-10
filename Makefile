PROJECT_ROOT_DIR = .
PROJECT_NAME ?= appcat
PROJECT_OWNER ?= vshn

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

OS := $(shell uname)
ifeq ($(OS), Darwin)
	sed ?= gsed
else
	sed ?= sed
endif

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
BIN_FILENAME ?= $(PROJECT_DIR)/appcat

## Stackgres CRDs
STACKGRES_VERSION ?= 1.4.3
STACKGRES_CRD_URL ?= https://gitlab.com/ongresinc/stackgres/-/raw/${STACKGRES_VERSION}/stackgres-k8s/src/common/src/main/resources/crds

## BUILD:go
go_bin ?= $(PWD)/.work/bin
$(go_bin):
	@mkdir -p $@

uname_s := $(shell uname -s)
ifeq ($(uname_s),Linux)
	distr_protoc := linux-x86_64
else
	distr_protoc := osx-universal_binary
endif

protoc_bin = $(go_bin)/protoc
$(protoc_bin): export GOBIN = $(go_bin)
$(protoc_bin): | $(go_bin)
	@echo "installing protocol buffers with dependencies"
	@git clone -q --depth 1 https://github.com/kubernetes/kubernetes.git .work/kubernetes
	@go install github.com/gogo/protobuf/protoc-gen-gogo@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@wget -q -O $(go_bin)/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v22.1/protoc-22.1-$(distr_protoc).zip
	@unzip $(go_bin)/protoc.zip -d .work
	@rm $(go_bin)/protoc.zip

-include docs/antora-preview.mk docs/antora-build.mk
-include package/package.mk
-include ci.mk

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: protobuf-gen
protobuf-gen: export PATH := $(go_bin):$(PATH)
protobuf-gen: $(protoc_bin)
	go run k8s.io/code-generator/cmd/go-to-protobuf@v0.26.3 \
		--packages=github.com/vshn/appcat/v4/apis/apiserver/v1 \
		--output-base=./.work/tmp \
		--go-header-file=./pkg/apiserver/hack/boilerplate.txt  \
        --apimachinery-packages='-k8s.io/apimachinery/pkg/util/intstr,-k8s.io/apimachinery/pkg/api/resource,-k8s.io/apimachinery/pkg/runtime/schema,-k8s.io/apimachinery/pkg/runtime,-k8s.io/apimachinery/pkg/apis/meta/v1,-k8s.io/apimachinery/pkg/apis/meta/v1beta1,-k8s.io/api/core/v1,-k8s.io/api/rbac/v1' \
        --proto-import=./.work/kubernetes/staging/src/ \
		--proto-import=./.work/kubernetes/vendor && \
    	mv ./.work/tmp/github.com/vshn/appcat/v4/apis/apiserver/v1/generated.pb.go ./apis/apiserver/v1/ && \
    	rm -rf ./.work/tmp

.PHONY: generate
generate: export PATH := $(go_bin):$(PATH)
generate:  get-crds generate-stackgres-crds protobuf-gen ## Generate code with controller-gen and protobuf.
	go version
	rm -rf apis/generated
	go run sigs.k8s.io/controller-tools/cmd/controller-gen paths="{./apis/v1/..., ./apis/vshn/..., ./apis/exoscale/..., ./apis/apiserver/..., ./apis/syntools/...}" object crd:crdVersions=v1,allowDangerousTypes=true output:artifacts:config=./apis/generated
	go generate ./...
	# Because yaml is such a fun and easy specification, we need to hack some things here.
	# Depending on the yaml parser implementation the equal sign (=) has special meaning, or not...
	# So we make it explicitly a string.
	find apis/generated/ -type f -exec $(sed) -i ':a;N;$$!ba;s/- =\n/- "="\n/g' {} \;
	rm -rf crds && cp -r apis/generated crds
	go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=appcat-sli-exporter paths="{./pkg/sliexporter/...}" output:artifacts:config=config/sliexporter/rbac
	go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=appcat-controller paths="{./pkg/controller/...}" output:rbac:stdout > config/controller/cluster-role.yaml
	go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=appcat-controller paths="{./pkg/apiserver/...}" output:rbac:stdout > config/apiserver/role.yaml
	go run sigs.k8s.io/controller-tools/cmd/controller-gen webhook paths="{./pkg/controller/...}" output:stdout > config/controller/webhooks.yaml

.PHONY: generate-stackgres-crds
generate-stackgres-crds:
	curl ${STACKGRES_CRD_URL}/SGDbOps.yaml?inline=false -o apis/stackgres/v1/sgdbops_crd.yaml
	yq -i e apis/stackgres/v1/sgdbops.yaml --expression ".components.schemas.SGDbOpsSpec=load(\"apis/stackgres/v1/sgdbops_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.spec"
	yq -i e apis/stackgres/v1/sgdbops.yaml --expression ".components.schemas.SGDbOpsStatus=load(\"apis/stackgres/v1/sgdbops_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.status"
	go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1 -generate=types -o apis/stackgres/v1/sgdbops.gen.go apis/stackgres/v1/sgdbops.yaml
	perl -i -0pe 's/\*struct\s\{\n\s\sAdditionalProperties\smap\[string\]string\s`json:"-"`\n\s}/map\[string\]string/gms' apis/stackgres/v1/sgdbops.gen.go

	curl ${STACKGRES_CRD_URL}/SGCluster.yaml?inline=false -o apis/stackgres/v1/sgcluster_crd.yaml
	yq -i e apis/stackgres/v1/sgcluster.yaml --expression ".components.schemas.SGClusterSpec=load(\"apis/stackgres/v1/sgcluster_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.spec"
	yq -i e apis/stackgres/v1/sgcluster.yaml --expression ".components.schemas.SGClusterStatus=load(\"apis/stackgres/v1/sgcluster_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.status"
	go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1 -generate=types -o apis/stackgres/v1/sgcluster.gen.go apis/stackgres/v1/sgcluster.yaml
	perl -i -0pe 's/\*struct\s\{\n\s\sAdditionalProperties\smap\[string\]string\s`json:"-"`\n\s}/map\[string\]string/gms' apis/stackgres/v1/sgcluster.gen.go

	# The generator for the pool config CRD unfortunately produces a broken result. However if we ever need to regenerate it in the future, please uncomment this.
	# curl ${STACKGRES_CRD_URL}/SGInstanceProfile.yaml?inline=false -o apis/stackgres/v1/sginstanceprofile_crd.yaml
	# yq -i e apis/stackgres/v1/sginstanceprofile.yaml --expression ".components.schemas.SGInstanceProfileSpec=load(\"apis/stackgres/v1/sginstanceprofile_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.spec"
	# go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1 -generate=types -o apis/stackgres/v1/sginstanceprofile.gen.go apis/stackgres/v1/sginstanceprofile.yaml
	# perl -i -0pe 's/\*struct\s\{\n\s\sAdditionalProperties\smap\[string\]string\s`json:"-"`\n\s}/map\[string\]string/gms' apis/stackgres/v1/sginstanceprofile.gen.go

	# The generator for the pool config CRD unfortunately produces a broken result. However if we ever need to regenerate it in the future, please uncomment this.
	# curl ${STACKGRES_CRD_URL}/SGPoolingConfig.yaml?inline=false -o apis/stackgres/v1/sgpoolconfigs_crd.yaml
	# yq -i e apis/stackgres/v1/sgpoolconfigs.yaml --expression ".components.schemas.SGPoolingConfigSpec=load(\"apis/stackgres/v1/sgpoolconfigs_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.spec"
	# yq -i e apis/stackgres/v1/sgpoolconfigs.yaml --expression ".components.schemas.SGPoolingConfigStatus=load(\"apis/stackgres/v1/sgpoolconfigs_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.status"
	# go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1 -generate=types -o apis/stackgres/v1/sgpoolconfigs.gen.go apis/stackgres/v1/sgpoolconfigs.yaml
	# perl -i -0pe 's/\*struct\s\{\n\s\sAdditionalProperties\smap\[string\]string\s`json:"-"`\n\s}/map\[string\]string/gms' apis/stackgres/v1/sgpoolconfigs.gen.go

	curl ${STACKGRES_CRD_URL}/SGObjectStorage.yaml?inline=false -o apis/stackgres/v1beta1/sgobjectstorage_crd.yaml
	yq -i e apis/stackgres/v1beta1/sgobjectstorage.yaml --expression ".components.schemas.SGObjectStorageSpec=load(\"apis/stackgres/v1beta1/sgobjectstorage_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.spec"
	go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1beta1 -generate=types -o apis/stackgres/v1beta1/sgobjectstorage.gen.go apis/stackgres/v1beta1/sgobjectstorage.yaml
	perl -i -0pe 's/\*struct\s\{\n\s\sAdditionalProperties\smap\[string\]string\s`json:"-"`\n\s}/map\[string\]string/gms' apis/stackgres/v1beta1/sgobjectstorage.gen.go


	go run sigs.k8s.io/controller-tools/cmd/controller-gen object paths=./apis/stackgres/v1/...
	go run sigs.k8s.io/controller-tools/cmd/controller-gen object paths=./apis/stackgres/v1beta1/...
	rm apis/stackgres/v1/*_crd.yaml

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: fmt vet ## All-in-one linting
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

##@ Build

.PHONY: build
build: export CGO_ENABLED = 0
build: generate fmt vet  ## Build manager binary.
build:
	@echo "GOOS=$$(go env GOOS) GOARCH=$$(go env GOARCH)"
	go build -o $(BIN_FILENAME)

.PHONY: test
test: ## Run tests
	go test ./... -count=1

.PHONY: kind-load-branch-tag
kind-load-branch-tag: ## load docker image with current branch tag into kind
	tag=$$(git rev-parse --abbrev-ref HEAD) && \
	kind load docker-image --name kindev ghcr.io/vshn/appcat:"$${tag////_}"

# Generate webhook certificates.
# This is only relevant when debugging.
# Component-appcat installs a proper certificate for this.
.PHONY: webhook-cert
webhook_key = .work/webhook/tls.key
webhook_cert = .work/webhook/tls.crt
webhook-cert: $(webhook_cert) ## Generate webhook certificates for out-of-cluster debugging in an IDE

$(webhook_key):
	mkdir -p .work/webhook
	ipsan="" && \
	if [[ $(webhook_service_name) =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$$ ]]; then \
		ipsan=", IP:$(webhook_service_name)"; \
	fi; \
	openssl req -x509 -newkey rsa:4096 -nodes -keyout $@ --noout -days 3650 -subj "/CN=$(webhook_service_name)" -addext "subjectAltName = DNS:$(webhook_service_name)$$ipsan"

$(webhook_cert): $(webhook_key)
	ipsan="" && \
	if [[ $(webhook_service_name) =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$$ ]]; then \
		ipsan=", IP:$(webhook_service_name)"; \
	fi; \
	openssl req -x509 -key $(webhook_key) -nodes -out $@ -days 3650 -subj "/CN=$(webhook_service_name)" -addext "subjectAltName = DNS:$(webhook_service_name)$$ipsan"


.PHONY: webhook-debug
webhook_service_name = host.docker.internal

webhook-debug: $(webhook_cert) ## Creates certificates, patches the webhook registrations and applies everything to the given kube cluster
webhook-debug:
	cabundle=$$(cat .work/webhook/tls.crt | base64) && \
	kubectl annotate validatingwebhookconfigurations.admissionregistration.k8s.io appcat-validation kubectl.kubernetes.io/last-applied-configuration- && \
	kubectl annotate validatingwebhookconfigurations.admissionregistration.k8s.io appcat-validation cert-manager.io/inject-ca-from- && \
	kubectl get validatingwebhookconfigurations.admissionregistration.k8s.io appcat-validation -oyaml | \
	yq e "with(.webhooks[]; .clientConfig.caBundle = \"$$cabundle\") | with(.webhooks[]; .clientConfig.url = \"https://$(webhook_service_name):9443\" + .clientConfig.service.path) | with(.webhooks[]; del(.clientConfig.service))"  | \
	kubectl replace -f - && \
	kubectl annotate validatingwebhookconfigurations.admissionregistration.k8s.io appcat-validation kubectl.kubernetes.io/last-applied-configuration-

.PHONY: clean
clean:
	rm -rf bin/ appcat .work/ docs/node_modules $docs_out_dir .public .cache apiserver.local.config apis/generated default.sock

get-crds:
	./hack/get_crds.sh https://github.com/crossplane-contrib/provider-helm provider-helm apis/release apis/helm
	./hack/get_crds.sh https://github.com/crossplane-contrib/provider-kubernetes provider-kubernetes apis/object/v1alpha2 apis/kubernetes
	# There is currently a bug with the serialization if `inline` and  `omitempty` are set: https://github.com/crossplane/function-sdk-go/issues/161
	$(sed) -i 's/inline,omitempty/inline/g' apis/helm/release/v1beta1/types.go
	# provider-sql needs manual fixes... Running this every time would break them.
	# The crossplane code generator only works if the code is valid, but the code is not valid until the code generator has run...
	#./hack/get_crds.sh https://github.com/crossplane-contrib/provider-sql provider-sql apis/ apis/sql
	#rm apis/sql/sql.go


.PHONY: api-bootstrap
api-bootstrap:
	go run ./hack/bootstrap/template.go ${API_FILE}

.PHONY: bootstrap
bootstrap: api-bootstrap generate ## API bootstrapping, create a new claim/composite API ready to be used

.PHONY: install-proxy
install-proxy:
	kubectl apply -f hack/functionproxy

.PHONY: render-diff
render-diff: docker-build-branchtag ## Render diff between the cluter in KUBECONF and the local branch
	hack/diff/compare.sh
