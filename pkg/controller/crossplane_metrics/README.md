# Crossplane Metrics Collector

This package provides a Prometheus metrics collector for generic Crossplane resources. It exposes an info-style metric `crossplane_resource_info` with labels containing resource metadata, status conditions, and custom Kubernetes labels.

## Features

- **Automatic XRD Discovery**: Automatically discovers all Composite Resource Definitions (XRDs) on the cluster
- **Extra Resources**: Support for monitoring additional resources like Helm Releases, ProviderConfigs, etc.
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

**CROSSPLANE_LABEL_MAPPING** (required): JSON map of Kubernetes labels to Prometheus label names

```bash
export CROSSPLANE_LABEL_MAPPING='{
  "crossplane.io/claim-namespace": "claim_namespace",
  "crossplane.io/claim-name": "claim_name",
  "appcat.vshn.io/organization": "organization",
  "appcat.vshn.io/sla": "sla"
}'
```

#### Optional Configuration

**CROSSPLANE_EXTRA_RESOURCES** (optional): JSON map of additional resource APIs to monitor beyond auto-discovered XRDs

Format: `{"apiVersion": ["resource1", "resource2"]}`

```bash
# Single API version with multiple resources
export CROSSPLANE_EXTRA_RESOURCES='{"helm.crossplane.io/v1beta1": ["releases"]}'

# Multiple API versions with resources
export CROSSPLANE_EXTRA_RESOURCES='{
  "helm.crossplane.io/v1beta1": ["releases"],
  "mysql.sql.crossplane.io/v1alpha1": ["databases", "users"]
}'
```

## How It Works

### Automatic XRD Discovery

The collector automatically discovers all Composite Resource Definitions (XRDs) on the cluster at startup by:

1. Listing all `compositeresourcedefinitions.apiextensions.crossplane.io/v1` resources
2. Extracting the group, versions, and plural resource names from each XRD
3. Only monitoring versions marked as `served: true`
4. Automatically collecting metrics for all discovered composite resources

This means you don't need to manually configure which XRDs to monitor - they're discovered automatically!

### Extra Resources

For resources that are not XRDs (like Helm Releases, ProviderConfigs, managed resources, etc.), you can specify them using `CROSSPLANE_EXTRA_RESOURCES`:

The format is a JSON map where the key is the full API version (`group/version`) and the value is an array of resource names (plural form).

Example:
```json
{
  "helm.crossplane.io/v1beta1": ["releases"],
  "mysql.sql.crossplane.io/v1alpha1": ["databases", "users"],
  "aws.upbound.io/v1beta1": ["buckets", "roles"]
}
```

This configuration tells the collector to monitor:
- `releases` from `helm.crossplane.io/v1beta1`
- `databases` and `users` from `mysql.sql.crossplane.io/v1alpha1`
- `buckets` and `roles` from `aws.upbound.io/v1beta1`

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
        - name: CROSSPLANE_LABEL_MAPPING
          value: |
            {
              "crossplane.io/claim-namespace": "claim_namespace",
              "crossplane.io/claim-name": "claim_name",
              "appcat.vshn.io/organization": "organization"
            }
        # Optional: Monitor additional resources beyond XRDs
        - name: CROSSPLANE_EXTRA_RESOURCES
          value: |
            {
              "helm.crossplane.io/v1beta1": ["releases"],
              "aws.upbound.io/v1beta1": ["providerconfigs"]
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
