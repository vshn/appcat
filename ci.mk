# Image URL to use all building/pushing image targets
IMG_TAG ?= latest
APP_NAME ?= appcat
ORG ?= vshn
GHCR_IMG ?= ghcr.io/$(ORG)/$(APP_NAME):$(IMG_TAG)
DOCKER_CMD ?= docker

# For alpine image it is required the following env before building the application
DOCKER_IMAGE_GOOS = linux
DOCKER_IMAGE_GOARCH = amd64

COMPONENT_REPO ?= https://github.com/vshn/component-appcat

.PHONY: docker-build
docker-build:
	env CGO_ENABLED=0 GOOS=$(DOCKER_IMAGE_GOOS) GOARCH=$(DOCKER_IMAGE_GOARCH) \
		go build -o ${BIN_FILENAME}
	docker build --platform $(DOCKER_IMAGE_GOOS)/$(DOCKER_IMAGE_GOARCH) -t ${GHCR_IMG} .

.PHONY: docker-build-branchtag
IMG_TAG =  $(shell git rev-parse --abbrev-ref HEAD | sed 's/\//_/g')
docker-build-branchtag: docker-build ## Build docker image with current branch name

.PHONY: docker-push
docker-push: docker-build ## Push docker image with the manager.
	docker push ${GHCR_IMG}

.PHONY: docker-push-branchtag
IMG_TAG =  $(shell git rev-parse --abbrev-ref HEAD | sed 's/\//_/g')
docker-push-branchtag: docker-build-branchtag docker-push ## Push docker image with current branch name

.PHONY: function-build
function-build: docker-build
	yq e '.spec.image="${GHCR_IMG}"' package/crossplane.yaml.template > package/crossplane.yaml
	rm -f package/*.xpkg
	go run github.com/crossplane/crossplane/cmd/crank@v1.16.0 xpkg build -f package --verbose --embed-runtime-image=${GHCR_IMG} -o package/package-function-appcat.xpkg
	git checkout package/crossplane.yaml

.PHONY: function-push-package
function-push-package: function-build
	go run github.com/crossplane/crossplane/cmd/crank@v1.16.0 xpkg push -f package/package-function-appcat.xpkg ghcr.io/vshn/appcat:${IMG_TAG}-func --verbose

.PHONY: function-build-branchtag
IMG_TAG =  $(shell git rev-parse --abbrev-ref HEAD | sed 's/\//_/g')
function-build-branchtag: docker-build-branchtag
	yq e '.spec.image="${GHCR_IMG}"' package/crossplane.yaml.template > package/crossplane.yaml
	rm -f package/*.xpkg
	go run github.com/crossplane/crossplane/cmd/crank@v1.16.0 xpkg build -f package --verbose --embed-runtime-image=${GHCR_IMG} -o package/package-function-appcat.xpkg
	git checkout package/crossplane.yaml

.PHONY: function-push-package-branchtag
IMG_TAG =  $(shell git rev-parse --abbrev-ref HEAD | sed 's/\//_/g')
function-push-package-branchtag: function-build-branchtag
	go run github.com/crossplane/crossplane/cmd/crank@v1.16.0 xpkg push -f package/package-function-appcat.xpkg ${GHCR_IMG}-func --verbose

.PHONY: create-component-pr
BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
create-component-pr:
	export CLONE_DIR=$$(mktemp -d) && \
	echo $$CLONE_DIR && \
	git clone ${COMPONENT_REPO} $$CLONE_DIR && \
	cd $$CLONE_DIR && \
	git checkout -b ${BRANCH} && \
	# This is a bit convoluted, but otherwise yq would strip out all
	# empty lines in the file.
	yq e '.parameters.appcat.images.${APP_NAME}.tag="${BRANCH}"' class/defaults.yml | diff -B class/defaults.yml - | patch class/defaults.yml -
	rm -rf $$CLONE_DIR
