//go:build generate

// Remove existing manifests

// Generate deepcopy methodsets and CRD manifests

// Generate crossplane-runtime methodsets (resource.Claim, etc)
//go:generate go run -tags generate github.com/crossplane/crossplane-tools/cmd/angryjet generate-methodsets --header-file=../../.github/boilerplate.go.txt ./...

package codey
