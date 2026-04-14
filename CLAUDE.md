# CLAUDE.md

## Project Overview

AppCat = Kubernetes app catalog managing cloud services via Crossplane composition functions. Four main components:

- **Controller** — Manages AppCat services w/ deletion protection, validation webhooks, SSH gateway
- **API Server** — Kubernetes aggregated API server for AppCat resources
- **gRPC Server** — Crossplane composition functions transforming claims into managed resources
- **SLI Exporter** — Monitors SLIs, exports Prometheus metrics

Managed services: PostgreSQL (StackGres + CloudNativePG), Redis, MariaDB, MinIO, Keycloak, Nextcloud, Forgejo, Garage, Codey.

## Git & Branching

- **Main branch**: `develop` (use for PRs and diff comparisons)
- No auto-commits — user handles committing
- No removing existing code unless explicitly asked

## Specialized Agents

Use as **coding buddies** — bounce ideas, get architectural advice, validate approaches, get service-specific context. Should **not** write code directly. Main session handles all code changes; agents for discussion and review.

| Agent | Domain |
|-------|--------|
| `vshnpostgres-expert` | PostgreSQL — StackGres (`vshnpostgres/`) and CNPG (`vshnpostgrescnpg/`), webhooks, backup/restore, HA, PgBouncer, SLI probes |
| `vshnredis-expert` | Redis — Sentinel HA, TLS, composition functions, webhooks, SLI probes, memory/eviction tuning |
| `mariadb-service-expert` | MariaDB — Galera clusters, ProxySQL, user management, webhooks, SLI probes |
| `vshnkeycloak-expert` | Keycloak — Helm deployment, PostgreSQL dependency, custom themes/providers/files, config redeployment, ingress, SLI probes |
| `vshnnextcloud-expert` | Nextcloud — Helm deployment, PostgreSQL dependency, Collabora Online, FQDN/ingress, backup w/ maintenance mode |
| `vshnforgejo-expert` | Forgejo — Helm deployment, SSH gateway (port allocation, Gateway API sharding), backup, mailer config |
| `vshngarage-expert` | Garage — S3-compatible object storage, Helm cluster deployment, bucket provisioning |
| `codey-expert` | Codey — managed Git hosting (`codey.io` API group), FQDN uniqueness validation, plan config |

MinIO deprecated, no dedicated agent.

## Build & Test

All build commands run in devcontainer: `ssh schedar-devcontainer.devpod`. Codebase at `/workspaces/schedar-devcontainer/appcat`.

```bash
make generate        # Generate CRDs, protobuf, RBAC
make build           # Build binary
make test            # Run all tests
make lint            # fmt + vet + diff check
make docker-build    # Build Docker image
make kind-load-branch-tag  # Load image into kind
```

### Running Tests

```bash
go test ./...                                          # All tests
go test ./pkg/comp-functions/functions/vshnpostgres/... # Specific package
go test ./pkg/apiserver/vshn/postgres/ -run TestBackup  # Specific test
go test -v ./...                                        # Verbose
```

### Testing Composition Functions with Crank

```bash
# Render locally
go run github.com/crossplane/crossplane/cmd/crank beta render xr.yaml composition.yaml functions.yaml -r

# Diff against live cluster
make render-diff

# With debug logging
make render-diff -e DEBUG=Development
```

## Architecture

### Directory Structure

- `apis/` — Kubernetes API types (Kubebuilder markers)
  - `vshn/v1/` — VSHN managed service types (PostgreSQL, Redis, MariaDB, MinIO, Keycloak, Nextcloud, Forgejo, Garage)
  - `cnpg/` — CloudNativePG API types
  - `codey/` — Codey service types
  - `exoscale/v1/` — Exoscale provider types (Kafka, MySQL, OpenSearch, PostgreSQL, Redis)
  - `stackgres/` — StackGres operator types
  - `generated/` — Auto-generated CRDs (no edit)
- `pkg/`
  - `comp-functions/` — Crossplane composition functions
    - `functions/` — Per-service function packages (see below)
    - `runtime/` — Shared helper library
  - `controller/` — Controller + webhook implementations
  - `apiserver/` — API server logic
  - `sliexporter/` — SLI monitoring + metrics
  - `maintenance/` — Maintenance ops (includes CNPG-specific maintenance)
- `cmd/` — Cobra CLI commands per component
- `config/` — Kubernetes manifests (controller, apiserver, sliexporter)
- `crds/` — Published CRDs (copied from `apis/generated/`)
- `test/` — Test fixtures, mocks, test clients
- `hack/` — Helper scripts + utilities

### Composition Functions

Each service has own package under `pkg/comp-functions/functions/`:

| Package | Service | Notes |
|---------|---------|-------|
| `vshnpostgres/` | PostgreSQL (StackGres) | Original implementation |
| `vshnpostgrescnpg/` | PostgreSQL (CloudNativePG) | Newer CNPG-based w/ extensions, backup, HA alerting |
| `vshnredis/` | Redis | |
| `vshnmariadb/` | MariaDB | |
| `vshnminio/` | MinIO | |
| `vshnkeycloak/` | Keycloak | |
| `vshnnextcloud/` | Nextcloud | |
| `vshnforgejo/` | Forgejo | Includes SSH gateway support |
| `vshngarage/` | Garage | Object storage |
| `buckets/` | Object storage buckets | |
| `spks/spksmariadb/` | MariaDB (StackGres) | StackGres-specific |
| `spks/spksredis/` | Redis (StackGres) | StackGres-specific |
| `common/` | Shared function utilities | |

**Adding new function**:
1. Create package under `pkg/comp-functions/functions/`
2. Implement `Transform()` using helpers from `runtime/`
3. Register in `cmd/functions.go`

### Webhooks

Validation webhooks in `pkg/controller/webhooks/`:

- **Per-service**: `postgresql.go`, `redis.go`, `mariadb.go`, `minio.go`, `keycloak.go`, `nextcloud.go`, `forgejo.go`, `codey.go`, `mysql.go`
- **Cross-cutting**: `deletionprotection.go`, `claim_deletionprotection.go`, `disk_downsize.go`, `objectbuckets.go`, `xobjectbuckets.go`
- **SSH Gateway**: `sshgateway/` — Port allocation, sharding, handler logic for Forgejo SSH access

### API Generation

- CRD generation: `controller-gen` via Kubebuilder markers
- Protobuf generation: `go-to-protobuf`
- CNPG CRD generation: `make generate-cnpg-crds`
- StackGres CRD generation: `make generate-stackgres-crds`
- **Always run `make generate` after modifying types in `apis/`**

### External CRD Integration

Project imports CRDs from external providers:
- `provider-helm`: Release APIs
- `provider-kubernetes`: Object APIs
- StackGres: PostgreSQL operator CRDs
- CloudNativePG: CNPG cluster CRDs

Run `make get-crds` to update external CRDs.

## Local Development

### DevContainer Setup

Project uses VS Code devcontainers w/ pre-configured kind cluster (kindev):
- Run `Dev Containers: Reopen in container` in VS Code
- Kindev config: `.kind/.kind/kind-config`
- Host kubeconfig: `.kind/.kind/kube-config`
- Setup from scratch: `make setup-kindev`

### Running Components Locally

**Setting host IP** (needed for controller/apiserver):
```bash
HOSTIP=$(docker inspect kindev-control-plane | jq '.[0].NetworkSettings.Networks.kind.Gateway') # macOS/Windows
# or: host.docker.internal (Docker Desktop) / host.lima.internal (Lima)
```

**Controller with webhooks**:
```bash
make webhook-debug -e webhook_service_name=$HOSTIP
go run . controller --certdir .work/webhook
```

**gRPC functions server**:
```bash
go run main.go --log-level 1 functions --insecure
make install-proxy  # Install function proxy in cluster
```

**API server**:
```bash
kubectl delete apiservice v1.api.appcat.vshn.io
kubectl -n syn-appcat delete svc appcat
go run . apiserver --secure-port=9443 \
  --kubeconfig=~/.kube/config \
  --authentication-kubeconfig=~/.kube/config \
  --authorization-kubeconfig=~/.kube/config \
  --feature-gates=APIPriorityAndFairness=false
```

## Key Patterns

### Crossplane Composition

Services = Crossplane Composites. Composition functions (gRPC) transform high-level claims into low-level managed resources. Function pipeline registered in `cmd/functions.go`.

### PostgreSQL: Two Implementations

- **StackGres** (`vshnpostgres/`): Original, uses StackGres operator
- **CloudNativePG** (`vshnpostgrescnpg/`): Newer, uses CNPG operator — extensions, major version upgrades, WAL storage, self-service restore, loadbalancer exposure

Both share same webhook (`postgresql.go`) and API types but differ in composition function logic and maintenance ops.

### SLI Exporter

- Histogram metric: `appcat_probes_seconds` w/ labels: service, namespace, name, sla, high_available, reason
- New prober: implement `Prober` interface in `pkg/sliexporter/probes/`
- New service: create controller in `pkg/sliexporter/`

## Entry Points

Main binary supports multiple subcommands (see `main.go`):
- `functions` — gRPC composition functions server (**default** if no command)
- `controller` — AppCat controller
- `apiserver` — Kubernetes API server
- `sliexporter` — SLI metrics exporter
- `maintenance` — Maintenance ops
- `slareport` — SLA reporting
- `hotfixreleaser` — Release hotfix tooling