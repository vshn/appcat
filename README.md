# appcat-apiserve

## Generate Kubernetes code

If you make changes to the auto generated code you'll need to run code generation.
This can be done with make:

```bash
make generate
```

## Building

See `make help` for a list of build targets.

* `make build`: Build binary for linux/amd64
* `make build -e GOOS=darwin -e GOARCH=arm64`: Build binary for macos/arm64
* `make docker-build`: Build Docker image for local environment

## Local development environment

You can setup a [kind]-based local environment with

```bash
make local-install
```

See [docs](./docs/modules/ROOT/pages/tutorials/dev-environment.md) for more details on the local environment setup.

Please be aware that the productive deployment of the appcat-apiserver may run on a different Kubernetes distribution than [kind].

[kind]: https://kind.sigs.k8s.io/
