.PHONY: package-function
package-function: docker-build
	rm -f package/*.xpkg
	go run github.com/crossplane/crossplane/cmd/crank xpkg build -f package --verbose --embed-runtime-image=${GHCR_IMG} -o package/package-function-appcat.xpkg

.PHONY: install-proxy
install-proxy: package-function
	id=$$(docker load < package/package-function-appcat.xpkg | cut -d ":" -f 3) && \
	docker tag $$id ghcr.io/vshn/appcat:latest-func && \
	kind load --name kindev docker-image ghcr.io/vshn/appcat:latest-func
	kubectl apply -f hack/functionproxy

.PHONY: push-function-package
push-function-package: package-function
	go run github.com/crossplane/crossplane/cmd/crank xpkg push -f package/package-function-appcat.xpkg ghcr.io/vshn/appcat:${IMG_TAG}-func --verbose
