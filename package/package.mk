.PHONY: package-function
package-function: docker-build
	yq e '.spec.image="${GHCR_IMG}"' package/crossplane.yaml.template > package/crossplane.yaml
	rm -f package/*.xpkg
	go run github.com/crossplane/crossplane/cmd/crank@v1.16.0 xpkg build -f package --verbose --embed-runtime-image=${GHCR_IMG} -o package/package-function-appcat.xpkg
	git checkout package/crossplane.yaml

.PHONY: install-proxy
install-proxy:
	kubectl apply -f hack/functionproxy

.PHONY: push-function-package
push-function-package: package-function
	go run github.com/crossplane/crossplane/cmd/crank@v1.16.0 xpkg push -f package/package-function-appcat.xpkg ghcr.io/vshn/appcat:${IMG_TAG}-func --verbose
