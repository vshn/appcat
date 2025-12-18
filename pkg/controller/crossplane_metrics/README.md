# Crossplane Metrics Collector

This package provides a Prometheus metrics collector for generic Crossplane resources. It exposes an info-style metric `crossplane_resource_info` with labels containing resource metadata, status conditions, and custom Kubernetes labels.

## Features

- **Dynamic Resource Discovery**: Collects metrics for any Crossplane CRD based on configuration
- **Status Conditions**: Automatically extracts and labels status conditions (e.g., `status_ready`, `status_synced`)
- **Label Mapping**: Maps Kubernetes labels to Prometheus labels for better querying
- **Provider Config**: Captures `providerConfigRef` information
- **Info Metric Pattern**: Uses gauge metrics with value `1` and rich labels (Prometheus info metric pattern)

## Usage

### Enabling the Collector

The collector is disabled by default. Enable it using the `--crossplane-metrics` flag:

```bash
./appcat controller --crossplane-metrics=true
```

### Configuration

The collector requires configuration via environment variables:

#### Required Configuration

The following configuration is required when `crossplane-metrics` is enabled:

**CROSSPLANE_RESOURCE_APIS** (required): JSON map of API versions to resource kinds to monitor

```bash
export CROSSPLANE_RESOURCE_APIS='{
  "vshn.appcat.vshn.io/v1": ["xvshnpostgresqls", "xvshnredis"],
  "exoscale.appcat.vshn.io/v1": ["exoscalepostgresqls", "exoscalemysqls"]
}'
```
**CROSSPLANE_LABEL_MAPPING** (required): JSON map of Kubernetes labels to Prometheus label names

```bash
export CROSSPLANE_LABEL_MAPPING='{
  "crossplane.io/claim-namespace": "claim_namespace",
  "crossplane.io/claim-name": "claim_name",
  "appcat.vshn.io/organization": "organization",
  "appcat.vshn.io/sla": "sla"
}'
```

## Metric Format

### Metric Name
`crossplane_resource_info`

### Labels

**Always Present:**
- `api_version`: Full API version (e.g., `vshn.appcat.vshn.io/v1`)
- `kind`: Resource kind (e.g., `xvshnpostgresqls`)
- `name`: Resource name

**Conditional Labels:**
- `status_<condition>`: Status condition reasons (e.g., `status_ready=Available`)
- `provider_config_ref`: Provider config reference name (if present)
- Custom labels based on `CROSSPLANE_LABEL_MAPPING`

### Example Metrics

```prometheus
# HELP crossplane_resource_info Generic Crossplane Resource Info
# TYPE crossplane_resource_info gauge
crossplane_resource_info{
  api_version="vshn.appcat.vshn.io/v1",
  kind="xvshnpostgresqls",
  name="my-postgres-instance",
  status_ready="Available",
  status_synced="ReconcileSuccess",
  claim_namespace="my-namespace",
  claim_name="my-postgres",
  provider_config_ref="exoscale-production"
} 1
```

## Example Queries

### Count resources by kind
```promql
count by (kind) (crossplane_resource_info)
```

### Find resources not ready
```promql
crossplane_resource_info{status_ready!="Available"}
```

### Resources per namespace
```promql
count by (claim_namespace) (crossplane_resource_info)
```

### Filter by provider config
```promql
crossplane_resource_info{provider_config_ref="exoscale-production"}
```

## Deployment Example

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: appcat-controller
spec:
  template:
    spec:
      containers:
      - name: controller
        image: ghcr.io/vshn/appcat:latest
        args:
          - controller
          - --crossplane-metrics=true
        env:
        - name: CROSSPLANE_RESOURCE_APIS
          value: |
            {
              "vshn.appcat.vshn.io/v1": ["xvshnpostgresqls", "xvshnredis", "xvshnmariadbs"],
              "exoscale.appcat.vshn.io/v1": ["exoscalepostgresqls"]
            }
        - name: CROSSPLANE_LABEL_MAPPING
          value: |
            {
              "crossplane.io/claim-namespace": "claim_namespace",
              "crossplane.io/claim-name": "claim_name",
              "appcat.vshn.io/organization": "organization"
            }
        ports:
        - name: metrics
          containerPort: 8080
```

### ServiceMonitor for Prometheus Operator

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: appcat-controller
spec:
  selector:
    matchLabels:
      app: appcat-controller
  endpoints:
  - port: metrics
    path: /metrics
    interval: 30s
```

## Required RBAC Permissions

The crossplane metrics controller requires read-only access to all crossplane resources it scrapes.

### Minimum Required Permissions

For each API group and resource kind configured in `CROSSPLANE_RESOURCE_APIS`, the controller needs:

- **Verbs**: `get`, `list`, `watch`
- **Resources**: The specific resource kinds being monitored (e.g., `xvshnpostgresqls`)

### Example ClusterRole

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane-metrics-reader
rules:
# For vshn.appcat.vshn.io/v1 resources
- apiGroups:
  - vshn.appcat.vshn.io
  resources:
  - xvshnpostgresqls
  - xvshnredis
  - xvshnmariadbs
  verbs:
  - get
  - list
  - watch
```

### ClusterRoleBinding

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: crossplane-metrics-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: crossplane-metrics-reader
subjects:
- kind: ServiceAccount
  name: appcat-controller
  namespace: appcat-system
```

### Important Notes

- **Read-only access**: The collector only reads resources and never modifies them
- **Cluster-scoped**: ClusterRole is required since the collector monitors resources across all namespaces and that might be cluster-scoped themselves.
- **Dynamic configuration**: Ensure RBAC permissions match the resources configured in `CROSSPLANE_RESOURCE_APIS`

## Architecture

The collector uses Kubernetes dynamic client to:
1. List resources for each configured API version and kind
2. Extract metadata, status conditions, and labels
3. Create Prometheus info metrics with rich labels
4. Serve metrics via the controller's existing metrics endpoint (`:8080/metrics`)

The collector is registered with the controller-runtime metrics registry and runs alongside other controller metrics.
