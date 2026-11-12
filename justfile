# appcat justfile

PROJECT_NAME := "appcat"
PROJECT_OWNER := "vshn"
STACKGRES_VERSION := "1.14.2"
CNPG_VERSION := "1.27.1"

go_bin := `pwd` + "/.work/bin"
project_dir := `pwd`
bin_filename := project_dir + "/appcat"

# Default recipe to display help
default:
    @just --list

# Display this help
help:
    @just --list

# Generate code with controller-gen and protobuf
generate: get-crds generate-stackgres-crds generate-cnpg-crds protobuf-gen
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH={{go_bin}}:$PATH
    go version
    rm -rf apis/generated
    go run sigs.k8s.io/controller-tools/cmd/controller-gen paths="{./apis/v1/..., ./apis/vshn/..., ./apis/exoscale/..., ./apis/apiserver/..., ./apis/syntools/..., ./apis/codey/...}" object crd:crdVersions=v1,allowDangerousTypes=true output:artifacts:config=./apis/generated
    go run sigs.k8s.io/controller-tools/cmd/controller-gen paths="./apis/cnpg/v1/..." object output:object:artifacts:config=./apis/cnpg/v1
    go generate ./...
    SED_CMD="sed"
    if [[ "$(uname)" == "Darwin" ]]; then SED_CMD="gsed"; fi
    find apis/generated/ -type f -exec $SED_CMD -i ':a;N;$!ba;s/- =\n/- "="\n/g' {} \;
    rm -rf crds && cp -r apis/generated crds
    go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=appcat-sli-exporter paths="{./pkg/sliexporter/...}" output:artifacts:config=config/sliexporter/rbac
    go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=appcat-controller paths="{./pkg/controller/...}" output:rbac:stdout > config/controller/cluster-role.yaml
    go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=appcat-controller paths="{./pkg/apiserver/...}" output:rbac:stdout > config/apiserver/role.yaml
    go run sigs.k8s.io/controller-tools/cmd/controller-gen webhook paths="{./pkg/controller/...}" output:stdout > config/controller/webhooks.yaml

# Generate stackgres CRDs
generate-stackgres-crds:
    #!/usr/bin/env bash
    set -euo pipefail
    STACKGRES_CRD_URL="https://gitlab.com/ongresinc/stackgres/-/raw/{{STACKGRES_VERSION}}/stackgres-k8s/src/common/src/main/resources/crds"
    curl ${STACKGRES_CRD_URL}/SGDbOps.yaml?inline=false -o apis/stackgres/v1/sgdbops_crd.yaml
    yq -i e apis/stackgres/v1/sgdbops.yaml --expression ".components.schemas.SGDbOpsSpec=load(\"apis/stackgres/v1/sgdbops_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.spec"
    yq -i e apis/stackgres/v1/sgdbops.yaml --expression ".components.schemas.SGDbOpsStatus=load(\"apis/stackgres/v1/sgdbops_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.status"
    go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1 -generate=types -o apis/stackgres/v1/sgdbops.gen.go apis/stackgres/v1/sgdbops.yaml
    perl -i -0pe 's/\*struct\s\{\n\s\sAdditionalProperties\smap\[string\]string\s`json:"-"`\n\s}/map\[string\]string/gms' apis/stackgres/v1/sgdbops.gen.go
    curl ${STACKGRES_CRD_URL}/SGObjectStorage.yaml?inline=false -o apis/stackgres/v1beta1/sgobjectstorage_crd.yaml
    yq -i e apis/stackgres/v1beta1/sgobjectstorage.yaml --expression ".components.schemas.SGObjectStorageSpec=load(\"apis/stackgres/v1beta1/sgobjectstorage_crd.yaml\").spec.versions[0].schema.openAPIV3Schema.properties.spec"
    go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1beta1 -generate=types -o apis/stackgres/v1beta1/sgobjectstorage.gen.go apis/stackgres/v1beta1/sgobjectstorage.yaml
    perl -i -0pe 's/\*struct\s\{\n\s\sAdditionalProperties\smap\[string\]string\s`json:"-"`\n\s}/map\[string\]string/gms' apis/stackgres/v1beta1/sgobjectstorage.gen.go
    go run sigs.k8s.io/controller-tools/cmd/controller-gen object paths=./apis/stackgres/v1/...
    go run sigs.k8s.io/controller-tools/cmd/controller-gen object paths=./apis/stackgres/v1beta1/...
    rm apis/stackgres/v1/*_crd.yaml

# Generate CNPG CRDs (currently mostly commented out in original)
generate-cnpg-crds:
    @echo "CNPG CRD generation mostly disabled in original Makefile"

# Generate protobuf code
protobuf-gen:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH={{go_bin}}:$PATH
    mkdir -p {{go_bin}}
    just install-protoc
    go run k8s.io/code-generator/cmd/go-to-protobuf@v0.26.3 \
        --packages=github.com/vshn/appcat/v4/apis/apiserver/v1 \
        --output-base=./.work/tmp \
        --go-header-file=./pkg/apiserver/hack/boilerplate.txt \
        --apimachinery-packages='-k8s.io/apimachinery/pkg/util/intstr,-k8s.io/apimachinery/pkg/api/resource,-k8s.io/apimachinery/pkg/runtime/schema,-k8s.io/apimachinery/pkg/runtime,-k8s.io/apimachinery/pkg/apis/meta/v1,-k8s.io/apimachinery/pkg/apis/meta/v1beta1,-k8s.io/api/core/v1,-k8s.io/api/rbac/v1' \
        --proto-import=./.work/kubernetes/staging/src/ \
        --proto-import=./.work/kubernetes/vendor
    mv ./.work/tmp/github.com/vshn/appcat/v4/apis/apiserver/v1/generated.pb.go ./apis/apiserver/v1/
    rm -rf ./.work/tmp

# Install protocol buffers compiler
install-protoc:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ ! -f {{go_bin}}/protoc ]; then
        echo "installing protocol buffers with dependencies"
        mkdir -p .work
        if [ ! -d .work/kubernetes ]; then
            git clone -q --depth 1 https://github.com/kubernetes/kubernetes.git .work/kubernetes
        fi
        export GOBIN={{go_bin}}
        go install github.com/gogo/protobuf/protoc-gen-gogo@latest
        go install golang.org/x/tools/cmd/goimports@latest
        DISTR_PROTOC="linux-x86_64"
        if [[ "$(uname -s)" != "Linux" ]]; then
            DISTR_PROTOC="osx-universal_binary"
        fi
        wget -q -O {{go_bin}}/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v22.1/protoc-22.1-${DISTR_PROTOC}.zip
        unzip {{go_bin}}/protoc.zip -d .work
        rm {{go_bin}}/protoc.zip
    fi

# Generate code with controller-gen and diff check
generate-with-diff-check: generate
    @echo 'Check for uncommitted changes ...'
    git diff --exit-code crds/

# Run go fmt against code
fmt:
    go fmt ./...

# Run go vet against code
vet:
    go vet ./...

# All-in-one linting
lint: fmt vet
    @echo 'Check for uncommitted changes ...'
    git diff --exit-code

# Build manager binary
build: generate fmt vet
    #!/usr/bin/env bash
    set -euo pipefail
    export CGO_ENABLED=0
    echo "GOOS=$(go env GOOS) GOARCH=$(go env GOARCH)"
    go build -o {{bin_filename}}

# Run tests
test:
    go test ./... -count=1

# Load docker image with current branch tag into kind
kind-load-branch-tag:
    #!/usr/bin/env bash
    set -euo pipefail
    tag=$(git rev-parse --abbrev-ref HEAD)
    kind load docker-image --name kindev ghcr.io/vshn/appcat:"$(echo $tag | sed 's#/#_#g')"

# Generate webhook certificates for out-of-cluster debugging in an IDE
webhook-cert:
    #!/usr/bin/env bash
    set -euo pipefail
    webhook_service_name="host.docker.internal"
    mkdir -p .work/webhook
    ipsan=""
    if [[ $webhook_service_name =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        ipsan=", IP:$webhook_service_name"
    fi
    openssl req -x509 -newkey rsa:4096 -nodes -keyout .work/webhook/tls.key --noout -days 3650 -subj "/CN=$webhook_service_name" -addext "subjectAltName = DNS:$webhook_service_name$ipsan"
    openssl req -x509 -key .work/webhook/tls.key -nodes -out .work/webhook/tls.crt -days 3650 -subj "/CN=$webhook_service_name" -addext "subjectAltName = DNS:$webhook_service_name$ipsan"

# Creates certificates, patches the webhook registrations and applies everything to the given kube cluster
webhook-debug: webhook-cert
    #!/usr/bin/env bash
    set -euo pipefail
    webhook_service_name="host.docker.internal"
    cabundle=$(cat .work/webhook/tls.crt | base64)
    kubectl annotate validatingwebhookconfigurations.admissionregistration.k8s.io appcat-validation kubectl.kubernetes.io/last-applied-configuration-
    kubectl annotate validatingwebhookconfigurations.admissionregistration.k8s.io appcat-validation cert-manager.io/inject-ca-from-
    kubectl get validatingwebhookconfigurations.admissionregistration.k8s.io appcat-validation -oyaml | \
    yq e "with(.webhooks[]; .clientConfig.caBundle = \"$cabundle\") | with(.webhooks[]; .clientConfig.url = \"https://$webhook_service_name:9443\" + .clientConfig.service.path) | with(.webhooks[]; del(.clientConfig.service))" | \
    kubectl replace -f -
    kubectl annotate validatingwebhookconfigurations.admissionregistration.k8s.io appcat-validation kubectl.kubernetes.io/last-applied-configuration-

# Clean build artifacts
clean:
    rm -rf bin/ appcat .work/ docs/node_modules .public .cache apiserver.local.config apis/generated default.sock

# Get CRDs from external providers
get-crds:
    ./hack/get_crds.sh https://github.com/vshn/provider-helm provider-helm apis/release apis/helm
    ./hack/get_crds.sh https://github.com/vshn/provider-kubernetes provider-kubernetes apis/object/v1alpha2 apis/kubernetes
    rm apis/kubernetes/v1alpha2/conversion.go
    SED_CMD="sed"
    if [[ "$(uname)" == "Darwin" ]]; then SED_CMD="gsed"; fi
    $SED_CMD -i 's/inline,omitempty/inline/g' apis/helm/release/v1beta1/types.go

# API bootstrapping, create a new claim/composite API ready to be used
bootstrap API_FILE:
    go run ./hack/bootstrap/template.go {{API_FILE}}
    just generate

# Install function proxy for development
install-proxy:
    kubectl apply -f hack/functionproxy

# Render diff between the cluster in KUBECONF and the local branch
render-diff DEBUG="":
    #!/usr/bin/env bash
    set -euo pipefail
    IMG_TAG=$(git rev-parse --abbrev-ref HEAD | sed 's/\//_/g')
    export IMG_TAG
    if ! docker pull $IMG; then
        echo "Image not pullable, would need to build it"
    fi
    hack/diff/compare.sh {{DEBUG}}

# Setup kindev in the .kind folder, will always create a new instance
setup-kindev:
    rm -rf .kind
    git clone https://github.com/vshn/kindev .kind
    cd .kind && just clean && just vshnall
