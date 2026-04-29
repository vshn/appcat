---
name: appcat-architecture
description: >
  AppCat framework architectural guidance and guardrails. Use when implementing,
  planning, or designing any AppCat feature — including new functions, backup,
  restore, upgrade, HA, maintenance, webhook, controller, or SLI probe changes.
  Triggers on: "implement", "add feature", "new function", "add backup",
  "add support for", service names (PostgreSQL, CNPG, Redis, MariaDB, Keycloak,
  Nextcloud, Forgejo, Garage). Do NOT use for general questions about the codebase.
---

# AppCat Architecture Guide

Inject these conventions and guardrails when implementing or planning AppCat changes. This skill provides framework-level knowledge — service-specific details come from the service agents listed below.

## Service Anatomy

A complete AppCat managed service consists of these components:

| Component | Location | Required |
|-----------|----------|----------|
| API types | `apis/vshn/v1/` | Yes |
| Composition functions | `pkg/comp-functions/functions/<service>/` | Yes |
| Function registration | `cmd/functions.go` | Yes |
| Validation webhook | `pkg/controller/webhooks/` | Yes |
| SLI probe | `pkg/sliexporter/probes/` | Yes |
| Maintenance ops | `pkg/maintenance/` | If applicable |
| Test fixtures | `test/functions/<service>/` | Yes |

When adding a new service, all required components must be present. Missing any creates an incomplete service that will fail in production.

## Composition Function Rules

These are the highest-priority guardrails. Violations cause production incidents.

### No Side Effects

Composition functions are **pure transforms**. They receive observed/desired state and return desired state. Nothing else.

- **No direct API calls** — no HTTP clients, no REST calls, no gRPC calls within functions
- **No kube client usage** — never instantiate or use a Kubernetes client
- External interactions go through **Crossplane providers** (provider-kubernetes, provider-helm, etc.)
- If a kube client is needed, that logic belongs in **`pkg/controller/`**, not in composition functions

### Early Returns Are Dangerous

An early return in a composition function can **silently drop resources from the desired state**. All resources set by prior functions in the pipeline are only preserved if the current function completes and writes them back.

Before adding an early return, verify:
- Will all previously-composed resources still be in the desired state?
- Could this drop a namespace, secret, or network policy?
- Is there a safer way to skip work without returning early?

### Name Length Limits

Kubernetes names have a **63-character limit**. When using `com.GetName` or composing names with suffixes:
- Instance name + suffix must stay under 63 characters
- Test with long instance names, not just short ones
- This is a common source of hard-to-debug failures in production

### General Patterns

- Use **`runtime/` helpers** — don't reimplement what's already there
- Follow existing patterns in the service's package
- Register every new function in **`cmd/functions.go`**
- Test with **`make render-diff`** to validate changes against a live cluster

## Code Organization

| Decision | Guideline |
|----------|-----------|
| Where to put shared logic | `pkg/comp-functions/functions/common/` — when 2+ services need it (e.g., `common/tcproute/` for TCP routing) |
| Where to put service logic | `pkg/comp-functions/functions/<service>/` — unique to one service |
| Framework helpers | `pkg/comp-functions/runtime/` — composition function infrastructure |
| Controller logic | `pkg/controller/` — anything needing a kube client or side effects |

**Default: start service-specific.** Promote to `common/` only when actual reuse appears. Premature sharing creates coupling.

## API Type Conventions

- Use **Kubebuilder markers** for CRD generation
- Types live in `apis/vshn/v1/` (or dedicated subdirectory for complex services)
- **Always run `make generate` after modifying types in `apis/`**
- External CRDs: update with `make get-crds`
- Generated code in `apis/generated/` — never edit directly

## Webhook Patterns

- **Validation webhooks**: enforce invariants — no version downgrades, no disk shrink, naming limits
- **Mutating webhooks**: set defaults, allocate resources (e.g., TCP port allocation for Gateway API)
- Check **cross-cutting webhooks** before creating new ones:
  - `deletionprotection.go` / `claim_deletionprotection.go` — deletion protection
  - `disk_downsize.go` — prevents disk shrinking
  - `objectbuckets.go` / `xobjectbuckets.go` — bucket validation
  - `tcpgateway/` — generic TCP port allocation + gateway sharding for XListenerSets (used by any service needing TCP routing)
- Webhook files live in `pkg/controller/webhooks/`

## TCP Routing (Gateway API)

Services needing TCP exposure (e.g., Forgejo SSH) use a shared two-layer system:

1. **Composition function layer** (`pkg/comp-functions/functions/common/tcproute/`): `AddTCPRoute()` creates XListenerSet + TCPRoute + NetworkPolicy via provider-kubernetes. Any service can call this with a `TCPRouteConfig`.
2. **Webhook layer** (`pkg/controller/webhooks/tcpgateway/`): service-agnostic mutating webhook allocates ports (via Leases) and shards across Gateways for any XListenerSet.

When adding TCP routing to a new service: use `tcproute.AddTCPRoute()` in the composition function. No webhook changes needed — the existing `tcpgateway/` handler covers all XListenerSets.

## Testing

- **Composition functions**: `make render-diff` (crank-based, diffs against live cluster)
- **Debug logging**: `make render-diff -e DEBUG=Development`
- **Unit tests**: `go test ./pkg/comp-functions/functions/<service>/...`
- **Test fixtures**: `test/functions/<service>/` — use as reference for expected input/output
- **Full suite**: `make test`
- After API type changes: `make generate` then `make test`

## SLI Exporter

- Standard metric: `appcat_probes_seconds` with labels: `service`, `namespace`, `name`, `sla`, `high_available`, `reason`
- Implement `Prober` interface in `pkg/sliexporter/probes/`
- Create controller in `pkg/sliexporter/` for new services
- Follow existing probers as templates

## Agent Routing

When implementation touches a specific service, **consult the relevant service agent** for domain-specific guidance. These agents are coding buddies — use them for advice, architecture discussion, and approach validation.

| Service | Agent | Key domain |
|---------|-------|------------|
| PostgreSQL (StackGres or CNPG) | `vshnpostgres-expert` | Two implementations, backup, HA, PgBouncer, major version upgrades |
| Redis | `vshnredis-expert` | Sentinel HA, TLS/mTLS, memory/eviction tuning |
| MariaDB | `mariadb-service-expert` | Galera clusters, ProxySQL, user management |
| Keycloak | `vshnkeycloak-expert` | Helm deployment, PostgreSQL dependency, custom themes/providers |
| Nextcloud | `vshnnextcloud-expert` | Helm deployment, Collabora Online, maintenance mode backup |
| Forgejo | `vshnforgejo-expert` | TCP gateway (port allocation, sharding), SSH access, backup |
| Garage | `vshngarage-expert` | S3-compatible object storage, Helm cluster setup |

**When to consult**: before making non-trivial changes to a service's composition functions, webhooks, or SLI probes. The agent knows the service's quirks and constraints that aren't obvious from the code.
