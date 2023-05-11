clean_targets += .clean-apis
OS := $(shell uname)
ifeq ($(OS), Darwin)
	sed ?= gsed
else
	sed ?= sed
endif

.PHONY: generate-xrd
generate-crd: ## Generates the CRDs using Kubebuilder, these CRDs will be used as a base for the XRDs by the component
	@rm -rf apis/generated
	@cd apis && go run sigs.k8s.io/controller-tools/cmd/controller-gen paths=./... crd:crdVersions=v1 output:artifacts:config=./generated
	@cd apis && go run sigs.k8s.io/controller-tools/cmd/controller-gen object paths=./...
	@cd apis && go generate ./...
	@# Because yaml is such a fun and easy specification, we need to hack some things here.
	@# Depending on the yaml parser implementation the equal sign (=) has special meaning, or not...
	@# So we make it explicitly a string.
	@$(sed) -i ':a;N;$$!ba;s/- =\n/- "="\n/g' apis/generated/vshn.appcat.vshn.io_vshnpostgresqls.yaml
	@rm -rf crds && cp -r apis/generated crds
.clean-apis:
	rm -rf apis/generated
