[![Build](https://img.shields.io/github/actions/workflow/status/vshn/appcat/test.yml?branch=master)](https://github.com/vshn/appcat/actions?query=workflow%3ATest)
![Go version](https://img.shields.io/github/go-mod/go-version/appuio/control-api)
![Kubernetes version](https://img.shields.io/badge/k8s-v1.24-blue)
[![Version](https://img.shields.io/github/v/release/appuio/control-api)](https://github.com/appuio/control-api/releases)
[![GitHub downloads](https://img.shields.io/github/downloads/vshn/appcat/total)](https://github.com/appuio/control-api/releases)

# AppCat

This repository has k8s tools to manage AppCat services.

## Architecture

```
.
├── apis
|   ├── appcat // API server related apis
|   ├── exoscale // exoscale apis
|   ├── v1 // common apis
|   ├── vshn // VSHN managed services apis
├── cmd // cobra command lines for each AppCat tool
|   ├── apiserver.go
|   ├── controller.go
|   ├── grpc.go
│   └── sliexporter.go
├── config // kubernetes resources
|   ├── apiserver
│   ├── controller
│   └── sliexporter // uses kustomize
├── crds // VSHN exposed resources from apis
├── docs
├── pkg // go code for each AppCat tool
|   ├── apiserver
|   ├── controller
|   ├── grpc
│   └── sliexporter
├── test // mocks, test cases, grps test client
├── Dockerfile
├── log.go
├── main.go
├── probes
├── README.md
└── tools.go
```

## Generate Kubernetes code, XRDs with Go / KubeBuilder

In `/apis` there is code in Go to generate the XRDs (composites) as this is in OpenAPI.
This code generates the OpenAPI scheme using [Kubebuilder](https://kubebuilder.io/).

See following pages for learning how to do that:
- https://kubebuilder.io/reference/generating-crd.html
- https://kubebuilder.io/reference/markers.html

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

You can setup a [kind]-based local environment with all dependencies in [kindev](https://github.com/vshn/kindev).

## API Server

The AppCat API Server facilitates work with AppCat services.

### Capabilities

The API Server is able to manage the following:

| RESOURCE           | DESCRIPTION                                             |
| ------------------ | ------------------------------------------------------- |
| `AppCat`           | Shows active AppCat services in a cluster               |
| `PotsgreSQLBackup` | Shows available backups of an AppCat PostgreSQL service |

### Debugging in IDE
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
apiserver --secure-port=9443 --kubeconfig=<your-home-path>/.kube/config --authentication-kubeconfig=<your-home-path>/.kube/config --authorization-kubeconfig=<your-home-path>/.kube/config --feature-gates=APIPriorityAndFairness=false
```

### Protobuf installation
Protocol Buffers (Protobuf) is a free and open-source cross-platform data format used to serialize structured data.
Kubernetes internally uses gRPC clients with protobuf serialization. APIServer objects when handled internally in K8S
need to implement protobuf interface. The implementation of the interface is done by [code-generator](https://github.com/kubernetes/code-generator). Two dependencies are required to use this tool [protoc](https://github.com/protocolbuffers/protobuf) and [protoc-gen-go](https://google.golang.org/protobuf/cmd/protoc-gen-go).


## Controller
The AppCat Controller resolves certain issues that cannot be achieved with crossplane.

### Capabilities

The controller manages and achieves the following:

| RESOURCE          | GOAL                        | DESCRIPTION                                  |
| ----------------- | --------------------------- | -------------------------------------------- |
| `XVSHNPostgreSQL` | Deletion Protection Support | Manages finalizers before and after deletion |

## SLI Exporter
### Metrics

The exporter exposes a histogram `appcat_probes_seconds` with four labels

* `service`, the service type that was probed (e.g. `VSHNPostgreSQL`)
* `namespace`, the namespace of the claim that was monitored
* `name`, the name of the claim that was monitored
* `reason`, if the probe was successful. Can either be `success`, `fail-timeout`, or `fail-unkown`

#### Adding a Prober

To add a prober for a new service, add a file in `pkg/sliexporter/probes/` that adds another implementation of the `Prober` interface.

#### Adding an AppCat service

There should be a separate controller per AppCat Claim type.
Add another controller for each service that should be probed in `pkg/sliexporter/`.

#### Adding SLA exceptions

If possible SLA exceptions should be implemented as a middleware for the `Prober` interface.


## Crossplane Composition Functions with gRPC Server

The gRPC Server that manages crossplane composition functions. The functions are found in `pkg/comp-functions/functions`
There package `pkg/comp-functions/runtime` contains a library with helper methods which helps with adding new functions.

### Add a new function-io

The framework is designed to easily add new composition functions to any AppCat service.
A function-io corresponds to one and only one composition thus multiple transformation go functions
can be added to a function-io.
For instance, in `vshn-postgres-func` there are multiple transformation go functions such as `url` or `alerting`.

To add a new function to PostgreSQL by VSHN:

- Create a new package under `./pkg/comp-functions/functions/` in case one does not exist for a composition.
- Implement a `Transform()` go function by using the helper functions from `runtime/desired.go` and `runtime/observed.go`.
- Add a new transform go function to the `images` variable in `./cmd/grpc.go`.

### Usage of gRPC server - local development + kind cluster

entrypoint to start working with gRPC server is to run:
```
go run main.go --log-level 1 start grpc --network tcp --socket ':9547' --devmode
```

This will start the GRPC server listening on a TCP port. Afterward you can configure the composition to use this connection:

```yaml
functions:
  - container:
      image: redis
      imagePullPolicy: IfNotPresent
      runner:
        endpoint: host.docker.internal:9547 # HERE
```

It's also possible to trigger fake request to gRPC server by client (to imitate Crossplane):
```
cd test/grpc-client
go run main.go start grpc
```

if You want to run gRPC server in local kind cluster, please use:
1. [kindev](https://github.com/vshn/kindev). In makefile replace target:
    1.
    ```
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
2. [component-appcat](https://github.com/vshn/component-appcat) please append [file](https://github.com/vshn/appcat/blob/master/tests/golden/vshn/appcat/appcat/21_composition_vshn_postgres.yaml) with:
    1.
    ```
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
