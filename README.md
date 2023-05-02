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
make local-debug

HOSTIP=$(docker inspect appcat-apiserver-v1.24.0-control-plane | jq '.[0].NetworkSettings.Networks.kind.Gateway')
# HOSTIP=host.docker.internal # On some docker distributions
# HOSTIP=host.lima.internal # On lima distributions

kind get kubeconfig --name appcat-apiserver-v1.24.0  > ~/.kube/config 

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

make apply-test-cases
```

After the above steps just run the API server via IDE with the following arguments.
```
apiserver --secure-port=9443 --kubeconfig=$HOME/.kube/config --authentication-kubeconfig=$HOME/.kube/config --authorization-kubeconfig=$HOME/.kube/config --tls-cert-file=dev/certificates/apiserver.crt --tls-private-key-file=dev/certificates/apiserver.key
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