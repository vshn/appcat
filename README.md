[![Build](https://img.shields.io/github/actions/workflow/status/vshn/appcat-apiserver/test.yml?branch=master)](https://github.com/vshn/appcat-apiserver/actions?query=workflow%3ATest)
![Go version](https://img.shields.io/github/go-mod/go-version/appuio/control-api)
![Kubernetes version](https://img.shields.io/badge/k8s-v1.24-blue)
[![Version](https://img.shields.io/github/v/release/appuio/control-api)](https://github.com/appuio/control-api/releases)
[![GitHub downloads](https://img.shields.io/github/downloads/vshn/appcat-apiserver/total)](https://github.com/appuio/control-api/releases)

# appcat-apiserver

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

## Debugging in IDE
To run the API server on your local machine you need to register the IDE running instance with kind cluster.
This can be achieved with the following guide.

The `externalName` needs to be changed to your specific host IP.
When running kind on Linux you can find it with `docker inspect`.
On some docker distributions the host IP is accessible via `host.docker.internal`.
For Lima distribution the host IP is accessible via `host.lima.internal`.

```bash
# Run this command in kindev -> https://github.com/vshn/kindev
make appcat-apiserver

HOSTIP=$(docker inspect kindev-control-plane | jq '.[0].NetworkSettings.Networks.kind.Gateway')
# HOSTIP=host.docker.internal # On some docker distributions
# HOSTIP=host.lima.internal # On lima distributions

kind get kubeconfig --name kindev  > ~/.kube/config 

cat <<EOF | sed -e "s/172.21.0.1/$HOSTIP/g" | kubectl apply -f -
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.api.appcat.vshn.io
  labels:
    api: appcat
    apiserver: "true"
spec:
  version: v1
  group: api.appcat.vshn.io
  insecureSkipTLSVerify: true
  groupPriorityMinimum: 2000
  service:
    name: appcat
    namespace: default
    port: 9443
  versionPriority: 10
---
apiVersion: v1
kind: Service
metadata:
  name: appcat
  namespace: default
spec:
  ports:
  - port: 9443
    protocol: TCP
    targetPort: 9443
  type: ExternalName
  externalName: 172.21.0.1 # Change to host IP
EOF
```

After the above steps just run the API server via IDE with the following arguments.
```
apiserver --secure-port=9443 --kubeconfig=<your-home-path>/.kube/config --authentication-kubeconfig=<your-home-path>/.kube/config --authorization-kubeconfig=<your-home-path>/.kube/config
```

## Protobuf installation
Protocol Buffers (Protobuf) is a free and open-source cross-platform data format used to serialize structured data.
Kubernetes internally uses gRPC clients with protobuf serialization. APIServer objects when handled internally in K8S
need to implement protobuf interface. The implementation of the interface is done by [code-generator](https://github.com/kubernetes/code-generator). Two dependencies are required to use this tool [protoc](https://github.com/protocolbuffers/protobuf) and [protoc-gen-go](https://google.golang.org/protobuf/cmd/protoc-gen-go).

## Controller
A postgres controller has been added to this API Server. The controller takes care of database deletion protection.
The controller insures via a finalizer in composite `XVSHNPostgreSQL` resource that the backups are still available
when the database instance is deleted. When deletion retention expires, the backups with the remaining resources
are cleaned up automatically.
This controller addition to this repository is a temporary solution until we move to a common AppCat repository.


## SLI Exporter
### Metrics

The exporter exposes a histogram `appcat_probes_seconds` with four labels

* `service`, the service type that was probed (e.g. `VSHNPostgreSQL`)
* `namespace`, the namespace of the claim that was monitored
* `name`, the name of the claim that was monitored
* `reason`, if the probe was successful. Can either be `success`, `fail-timeout`, or `fail-unkown`

### Architecture

```
.
├── config // Kustomize files to deploy the exporter
├── controllers
│   └── vshnpostgresql_controller.go // starts probes for VSHNPostgreSQL claims
├── Dockerfile
├── main.go
├── probes
│   ├── manager.go // manages all started probes and exposes metrics
│   └── postgresql.go // prober implementation for postgresql
├── README.md
└── tools.go
```

#### Adding a Prober

To add a prober for a new service, add a file in `probes/` that adds another implementation of the `Prober` interface.

#### Adding an AppCat service

There should be a separate controller per AppCat Claim type.
Add another controller for each service that should be probed.

#### Adding SLA exceptions

If possible SLA exceptions should be implemented as a middleware for the `Prober` interface.


## Crossplane Composition Functions with GRPS Server

```
.
├── docs
├── functions
│   ├── vshn-common-func
│   ├── vshn-postgres-func
│   └── vshn-redis-func
├── kind
├── runtime
└── test
```

- `./docs` contains relevant documentation in regard to this repository
- `./functions` contains the actual logic for each function-io. Each function-io can have multiple transformation go functions
- `./runtime` contains a library with helper methods which helps with adding new functions.
- `./kind` contains relevant files for local dev cluster
- `./test` contains test files

Check out the docs to understand how functions from this repository work.

### Add a new function-io

The framework is designed to easily add new composition functions to any AppCat service.
A function-io corresponds to one and only one composition thus multiple transformation go functions
can be added to a function-io.
For instance, in `vshn-postgres-func` there are multiple transformation go functions such as `url` or `alerting`.


To add a new function to PostgreSQL by VSHN:

- Create a new package under `./functions/`.
- Create a go file and add a new transform go function to the list in `./cmd/<your-new-function-io>`.
- Implement the actual `Transform()` go function by using the helper functions from `runtime/desired.go` and `runtime/observed.go`.
- Register the transform go function in the `main.go`.
- Create a new app.go under `./cmd/<your-new-function-io>` and define a new `AppInfo` object.

This architecture allows us to run all the functions with a single command. But for debugging and development purpose it's possible to run each function separately, by using the `--function` flag.

### Manually testing a function
To test a function you can leverage the FunctionIO file in the `./test` folder.

`cat test/function-io.yaml | go run cmd/vshn-postgres-func/main.go --function myfunction > test.yaml`


### Usage of gRPC server - local development + kind cluster

entrypoint to start working with gRPC server is to run:
```
go run main.go -socket default.sock
```

it will create a socket file in Your local directory which is easier for development - no need to set permissions and directory structure.

It's also possible to trigger fake request to gRPC server by client (to imitate Crossplane):
```
cd test/grpc-client
go run main.go
```

if You want to run gRPC server in local kind cluster, please use:
1. [kindev](https://github.com/vshn/kindev). In makefile replace target:
    1. ```
      $(crossplane_sentinel): export KUBECONFIG = $(KIND_KUBECONFIG)
      $(crossplane_sentinel): kind-setup local-pv-setup
      # below line loads image to kind
	  kind load docker-image --name kindev ghcr.io/vshn/appcat-comp-functions
	  helm repo add crossplane https://charts.crossplane.io/stable
	  helm upgrade --install crossplane --create-namespace --namespace syn-crossplane crossplane/crossplane \
	  --set "args[0]='--debug'" \
	  --set "args[1]='--enable-composition-functions'" \
	  --set "args[2]='--enable-environment-configs'" \
	  --set "xfn.enabled=true" \
	  --set "xfn.args={--debug}" \
	  --set "xfn.image.repository=ghcr.io/vshn/appcat-comp-functions" \
	  --set "xfn.image.tag=latest" \
	  --wait
	  @touch $@   
      ```
2. [component-appcat](https://github.com/vshn/component-appcat) please append [file](https://github.com/vshn/component-appcat/blob/master/tests/golden/vshn/appcat/appcat/21_composition_vshn_postgres.yaml) with:
    1.   ```
        compositeTypeRef:
          apiVersion: vshn.appcat.vshn.io/v1
          kind: XVSHNPostgreSQL
        # we have to add functions declaration to postgresql
        functions:
          - container:
              image: postgresql
              runner:
                endpoint: unix-abstract:crossplane/fn/default.sock
            name: pgsql-func
            type: Container
        resources:
          - base:
            apiVersion: kubernetes.crossplane.io/v1alpha1
        ```

That's all - You can now run Your claims. This documentation and above workaround is just temporary solution, it should disappear once we actually implement composition functions. 


## Generate XRDs with Go / KubeBuilder

In `/apis` there is code in Go to generate the XRDs (composites) as this is in OpenAPI.
This code generates the OpenAPI scheme using [Kubebuilder](https://kubebuilder.io/).

See following pages for learning how to do that:
- https://kubebuilder.io/reference/generating-crd.html
- https://kubebuilder.io/reference/markers.html

To run the composition generator, run `make generate-crd`.
You need to have `go` installed for this to work.

After that, you are able to update the golden files for the component: `make gen-golden-all`.
