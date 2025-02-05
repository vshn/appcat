[![Build](https://img.shields.io/github/actions/workflow/status/vshn/appcat/test.yml?branch=master)](https://github.com/vshn/appcat/actions?query=workflow%3ATest)
![Go version](https://img.shields.io/github/go-mod/go-version/vshn/appcat)
![Kubernetes version](https://img.shields.io/badge/k8s-v1.24-blue)
[![Version](https://img.shields.io/github/v/release/vshn/appcat)](https://github.com/vshn/appcat/releases)
[![GitHub downloads](https://img.shields.io/github/downloads/vshn/appcat/total)](https://github.com/vshn/appcat/releases)

# AppCat

This repository has k8s tools to manage AppCat services.

Documentation: https://vshn.github.io/appcat

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

## Getting started with devcontainers

This project contains a `.devcontainer` folder, which enables devcontainer support in vscode and other IDEs.
Make sure you've installed the dev containers extension for vscode.

Then open the command palette and do `Dev Containers: Reopen in container`.
The first time doing this will take quite some as it builds the whole container.
After building the container it will spin up kindev, which will also take some time.

Once it's finished there should be a ready to go dev environment to use.

The container will contain:
* go
* helm
* kubectl
* yq
* kubecolor

Additionally, it will install some useful extensions for vscode to make development easier.

Kindev will be installed in the `.kind` folder.
Vscode handles all port-forwarding automagically, so kindev and all endpoints will be available all the same from the host.
Simply point `KUBECONFIG` to `.kind/.kind/kind-config` and use `kubectl` as usual.

> [!NOTE]
> On linux, Podman and rootless Docker have been found to cause issues.  
> Ensure that you are using **rootful Docker** to save yourself loads of trouble.  
>
> Additionally, on Fedora, you might encounter issues with `iptables`.  
> Execute the following on the host prior to building the devcontainer:
> ```bash
> sudo dnf install -y iptables-legacy
> sudo modprobe ip_tables
> echo 'ip_tables' | sudo tee /etc/modules
> ```
>
> *To further debug dind, consult `/tmp/dockerd.log` in the container.*
> Please refer to official documentation under: https://docs.docker.com/engine/install/ 

### Devcontainer customizations

It's possible to customize the devcontainer.
By setting `"terminal.integrated.defaultProfile.linux": "zsh"` in the vscode config, it's possible to switch the default shell to zsh.

It's also possible to provide your own dotfiles. They will be installed after the kindev setup has finished.
For that, simply write a small script that contains all your desired configurations and put it in a publicly available repository.
Here's an example repo: https://github.com/lugoues/vscode-dev-containers-dotfiles

After that set this configuration in vscode:
```
{
  "dotfiles.repository": "your-github-id/your-dotfiles-repo",
  "dotfiles.targetPath": "~/dotfiles",
  "dotfiles.installCommand": "install.sh"
}
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

## Controller
The AppCat Controller resolves certain issues that cannot be achieved with crossplane.

### Capabilities

The controller manages and achieves the following:

| RESOURCE              | GOAL                                            | DESCRIPTION                                  |
| --------------------- | ----------------------------------------------- | -------------------------------------------- |
| `XVSHNPostgreSQL`     | Deletion Protection Support                     | Manages finalizers before and after deletion |
| `validation webhooks` | Provide validation webhooks for AppCat services | See Goal                                     |

### Local Webhook debugging

**This assumes that you've already applied the vshn golden test in component-appcat!**

Also you might need to set the right host ip:
```bash
HOSTIP=$(docker inspect kindev-control-plane | jq '.[0].NetworkSettings.Networks.kind.Gateway') # On kind MacOS/Windows
HOSTIP=host.docker.internal # On Docker Desktop distributions
HOSTIP=host.lima.internal # On Lima backed Docker distributions
For Linux users: `ip -4 addr show dev docker0 | grep inet | awk -F' ' '{print $2}' | awk -F'/' '{print $1}'`
```

Then you can apply the webhook registration changes:
```bash
export KUBECONFIG=...
make webhook-debug -e webhook_service_name=$HOSTIP
```

Then run the controller with the correct certdir flag:
```
go run . controller --certdir .work/webhook
```

Creating a PostgreSQL or Redis instance will now go through the Webhook instance.

## SLI Exporter
### Metrics

The exporter exposes a histogram `appcat_probes_seconds` with five labels

* `service`, the service type that was probed (e.g. `VSHNPostgreSQL`)
* `namespace`, the namespace of the claim that was monitored
* `name`, the name of the claim that was monitored
* `sla`, the service level. Can either be `besteffort` or `guaranteed`
* `high_available`, whether this instance is high available (1 or more nodes)
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
go run main.go --log-level 1 functions --insecure
```

This will start the GRPC server listening on a TCP port. Afterward you can configure the composition to use this connection in the component:

```yaml
# HOSTIP=$(docker inspect kindev-control-plane | jq '.[0].NetworkSettings.Networks.kind.Gateway') # On kind MacOS/Windows
        # HOSTIP=host.docker.internal # On Docker Desktop distributions
        # HOSTIP=host.lima.internal # On Lima backed Docker distributions
        # For Linux users: `ip -4 addr show dev docker0 | grep inet | awk -F' ' '{print $2}' | awk -F'/' '{print $1}'`
services:
  vshn:
    enabled: true
    externalDatabaseConnectionsEnabled: true
    emailAlerting:
      enabled: true
      smtpPassword: "whatever"
    postgres:
      grpcEndpoint: $HOSTIP:9547
      proxyFunction: true
```

Each service should have these params configured.

If you prefer to set the proxy endpoint directly in the composition, it's passed via the `input.data.proxyEndpoint` field.

Then install the function proxy in kind:

```bash
make install-proxy
```

It's also possible to trigger fake request to gRPC server via the crank renderer:
```
go run github.com/crossplane/crossplane/cmd/crank beta render xr.yaml composition.yaml functions.yaml -r
```
Crank will return a list of all the objects this specific request would have produced, including the result messages.

Please have a look at the `hack/` folder for an example.

### Generate a Diff against a live version

Make sure you have the `KUBECONFIG` exported for the cluster you want to run the diff.
This can be kindev or even another live cluster.
Also make sure, that AppCat is checked out at the branch you want the diff of.

Then run `make render-diff`.

This will:
* Check the live cluster which version AppCat and PnT func version are running
* Apply them to the local functions template
* Get all composites and their respective state, by downloading all the managed resources
* Use `crank render` to render the resulting list of managed resources
* Switch the functions template to the current branch
* Run `crank` again with the state already downloaded, to avoid any unintended diffs
* Use `dyff` to generate the diffs between both results

### Generate diff against live version with debugger

If you want to print the diff while using the debugger you can simply do this:

* Start the local debugging session as described
* Run `make render-diff -e DEBUG=Development`

NOTE: `crank render` has a 60s timeout, so you might run into it, if your debugging takes longer

# Run API Server locally
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
