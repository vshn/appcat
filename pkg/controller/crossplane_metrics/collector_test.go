package crossplane_metrics

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

func TestNewMetricsCollector(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	labelMappings := map[string]string{"app": "application"}
	extraResources := map[string][]string{}
	log := logr.Discard()

	collector := NewMetricsCollector(client, labelMappings, extraResources, log)

	assert.NotNil(t, collector)
	assert.Equal(t, client, collector.client)
	assert.Equal(t, labelMappings, collector.labelMappings)
	assert.Equal(t, extraResources, collector.extraResources)
}

func TestMetricsCollector_Describe(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	collector := NewMetricsCollector(client, nil, nil, logr.Discard())

	ch := make(chan *prometheus.Desc, 1)
	go func() {
		collector.Describe(ch)
		close(ch)
	}()

	desc := <-ch
	assert.NotNil(t, desc)
	assert.Contains(t, desc.String(), "crossplane_resource_info")
}

func TestMetricsCollector_Collect_Success(t *testing.T) {
	scheme := runtime.NewScheme()
	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name":      "test-resource",
				"namespace": "default",
				"labels": map[string]any{
					"app":         "myapp",
					"environment": "prod",
				},
			},
			"spec": map[string]any{
				"providerConfigRef": map[string]any{
					"name": "default-provider",
				},
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"reason": "Available",
						"status": "True",
					},
					map[string]any{
						"type":   "Synced",
						"reason": "ReconcileSuccess",
						"status": "True",
					},
				},
			},
		},
	}

	client := fake.NewSimpleDynamicClient(scheme, resource)
	labelMappings := map[string]string{
		"app":         "application",
		"environment": "env",
	}

	collector := NewMetricsCollector(client, labelMappings, nil, logr.Discard())
	// Manually set resourceAPIs for testing (bypassing auto-discovery)
	collector.xrds = map[string][]string{
		"example.com/v1": {"testresources"},
	}

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metrics := []prometheus.Metric{}
	for m := range ch {
		metrics = append(metrics, m)
	}

	require.Len(t, metrics, 1)

	// Validate the metric
	metric := metrics[0]
	pb := &dto.Metric{}
	err := metric.Write(pb)
	require.NoError(t, err)

	// Check that the metric value is 1 (info metric)
	assert.Equal(t, float64(1), pb.GetGauge().GetValue())

	// Validate labels
	labels := make(map[string]string)
	for _, label := range pb.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	assert.Equal(t, "example.com/v1", labels["api_version"])
	assert.Equal(t, "testresources", labels["kind"])
	assert.Equal(t, "test-resource", labels["name"])
	assert.Equal(t, "myapp", labels["application"])
	assert.Equal(t, "prod", labels["env"])
	assert.Equal(t, "Available", labels["status_ready"])
	assert.Equal(t, "ReconcileSuccess", labels["status_synced"])
	assert.Equal(t, "default-provider", labels["provider_config_ref"])
}

func TestMetricsCollector_Collect_MinimalResource(t *testing.T) {
	scheme := runtime.NewScheme()
	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name": "minimal-resource",
			},
		},
	}

	client := fake.NewSimpleDynamicClient(scheme, resource)

	collector := NewMetricsCollector(client, nil, nil, logr.Discard())
	// Manually set resourceAPIs for testing (bypassing auto-discovery)
	collector.xrds = map[string][]string{
		"example.com/v1": {"testresources"},
	}

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metrics := []prometheus.Metric{}
	for m := range ch {
		metrics = append(metrics, m)
	}

	require.Len(t, metrics, 1)

	// Validate the metric
	metric := metrics[0]
	pb := &dto.Metric{}
	err := metric.Write(pb)
	require.NoError(t, err)

	// Validate labels
	labels := make(map[string]string)
	for _, label := range pb.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	assert.Equal(t, "example.com/v1", labels["api_version"])
	assert.Equal(t, "testresources", labels["kind"])
	assert.Equal(t, "minimal-resource", labels["name"])
	// No status conditions or provider config should be present
	assert.NotContains(t, labels, "status_ready")
	assert.NotContains(t, labels, "provider_config_ref")
}

func TestMetricsCollector_Collect_MultipleResources(t *testing.T) {
	scheme := runtime.NewScheme()
	resource1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name": "resource-1",
			},
		},
	}
	resource2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name": "resource-2",
			},
		},
	}

	client := fake.NewSimpleDynamicClient(scheme, resource1, resource2)

	collector := NewMetricsCollector(client, nil, nil, logr.Discard())
	// Manually set resourceAPIs for testing (bypassing auto-discovery)
	collector.xrds = map[string][]string{
		"example.com/v1": {"testresources"},
	}

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metrics := []prometheus.Metric{}
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Len(t, metrics, 2)
}

func TestMetricsCollector_Collect_InvalidAPIVersion(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())

	collector := NewMetricsCollector(client, nil, nil, logr.Discard())
	// Manually set resourceAPIs for testing (bypassing auto-discovery)
	collector.xrds = map[string][]string{
		"invalid-api-version": {"testresources"},
	}

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metrics := []prometheus.Metric{}
	for m := range ch {
		metrics = append(metrics, m)
	}

	// Should not collect any metrics due to invalid API version
	assert.Len(t, metrics, 0)
}

func TestMetricsCollector_Collect_EmptyLabelValue(t *testing.T) {
	scheme := runtime.NewScheme()
	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name": "test-resource",
				"labels": map[string]any{
					"app":   "myapp",
					"empty": "",
				},
			},
		},
	}

	client := fake.NewSimpleDynamicClient(scheme, resource)
	labelMappings := map[string]string{
		"app":   "application",
		"empty": "empty_label",
	}

	collector := NewMetricsCollector(client, labelMappings, nil, logr.Discard())
	// Manually set resourceAPIs for testing (bypassing auto-discovery)
	collector.xrds = map[string][]string{
		"example.com/v1": {"testresources"},
	}

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metrics := []prometheus.Metric{}
	for m := range ch {
		metrics = append(metrics, m)
	}

	require.Len(t, metrics, 1)

	// Validate the metric
	metric := metrics[0]
	pb := &dto.Metric{}
	err := metric.Write(pb)
	require.NoError(t, err)

	// Validate labels
	labels := make(map[string]string)
	for _, label := range pb.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	assert.Equal(t, "myapp", labels["application"])
	// Empty labels should not be included
	assert.NotContains(t, labels, "empty_label")
}

func TestMetricsCollector_Collect_NoMatchingK8sLabels(t *testing.T) {
	scheme := runtime.NewScheme()
	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name": "test-resource",
				"labels": map[string]any{
					"other": "label",
				},
			},
		},
	}

	client := fake.NewSimpleDynamicClient(scheme, resource)
	labelMappings := map[string]string{
		"app": "application",
	}

	collector := NewMetricsCollector(client, labelMappings, nil, logr.Discard())
	// Manually set resourceAPIs for testing (bypassing auto-discovery)
	collector.xrds = map[string][]string{
		"example.com/v1": {"testresources"},
	}

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metrics := []prometheus.Metric{}
	for m := range ch {
		metrics = append(metrics, m)
	}

	require.Len(t, metrics, 1)

	// Validate the metric
	metric := metrics[0]
	pb := &dto.Metric{}
	err := metric.Write(pb)
	require.NoError(t, err)

	// Validate labels
	labels := make(map[string]string)
	for _, label := range pb.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	// Should not have the mapped label since the k8s label doesn't exist
	assert.NotContains(t, labels, "application")
}

func TestMetricsCollector_collectResource_MalformedConditions(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name": "test-resource",
			},
			"status": map[string]any{
				"conditions": []any{
					// Malformed condition - not a map
					"invalid-condition",
					// Valid condition
					map[string]any{
						"type":   "Ready",
						"reason": "Available",
					},
				},
			},
		},
	}

	collector := NewMetricsCollector(nil, nil, nil, logr.Discard())

	ch := make(chan prometheus.Metric, 10)
	err := collector.collectResource(ch, resource, "example.com/v1", "testresources")

	require.NoError(t, err)

	// Should still create a metric, just skipping the malformed condition
	close(ch)
	metrics := []prometheus.Metric{}
	for m := range ch {
		metrics = append(metrics, m)
	}

	require.Len(t, metrics, 1)

	// Validate the metric has the valid condition
	metric := metrics[0]
	pb := &dto.Metric{}
	err = metric.Write(pb)
	require.NoError(t, err)

	labels := make(map[string]string)
	for _, label := range pb.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	assert.Equal(t, "Available", labels["status_ready"])
}

func TestMetricsCollector_collectResource_ConditionWithoutType(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name": "test-resource",
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"reason": "NoType",
					},
				},
			},
		},
	}

	collector := NewMetricsCollector(nil, nil, nil, logr.Discard())

	ch := make(chan prometheus.Metric, 10)
	err := collector.collectResource(ch, resource, "example.com/v1", "testresources")

	require.NoError(t, err)
}

func TestMetricsCollector_collectResource_ConditionWithoutReason(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name": "test-resource",
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type": "Ready",
					},
				},
			},
		},
	}

	collector := NewMetricsCollector(nil, nil, nil, logr.Discard())

	ch := make(chan prometheus.Metric, 10)
	err := collector.collectResource(ch, resource, "example.com/v1", "testresources")

	require.NoError(t, err)
}

func TestMetricsCollector_Collect_MultipleAPIVersionsAndKinds(t *testing.T) {
	scheme := runtime.NewScheme()
	resource1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TypeA",
			"metadata": map[string]any{
				"name": "resource-a",
			},
		},
	}
	resource2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TypeB",
			"metadata": map[string]any{
				"name": "resource-b",
			},
		},
	}
	resource3 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "other.com/v1",
			"kind":       "TypeC",
			"metadata": map[string]any{
				"name": "resource-c",
			},
		},
	}

	client := fake.NewSimpleDynamicClient(scheme, resource1, resource2, resource3)

	collector := NewMetricsCollector(client, nil, nil, logr.Discard())
	// Manually set resourceAPIs for testing (bypassing auto-discovery)
	collector.xrds = map[string][]string{
		"example.com/v1": {"typeas", "typebs"},
		"other.com/v1":   {"typecs"},
	}

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metrics := []prometheus.Metric{}
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Len(t, metrics, 3)
}

func TestMetricsCollector_collectResource_StatusConditionTypeLowercase(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]any{
				"name": "test-resource",
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"reason": "Available",
					},
				},
			},
		},
	}

	collector := NewMetricsCollector(nil, nil, nil, logr.Discard())

	ch := make(chan prometheus.Metric, 10)
	err := collector.collectResource(ch, resource, "example.com/v1", "testresources")
	require.NoError(t, err)

	close(ch)
	metrics := []prometheus.Metric{}
	for m := range ch {
		metrics = append(metrics, m)
	}

	require.Len(t, metrics, 1)

	// Validate the metric
	metric := metrics[0]
	pb := &dto.Metric{}
	err = metric.Write(pb)
	require.NoError(t, err)

	labels := make(map[string]string)
	for _, label := range pb.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	// Verify the condition type is lowercased
	assert.Equal(t, "Available", labels["status_ready"])
}

func createTestClient(resources ...*unstructured.Unstructured) dynamic.Interface {
	scheme := runtime.NewScheme()
	objects := make([]runtime.Object, len(resources))
	for i, r := range resources {
		objects[i] = r
	}
	return fake.NewSimpleDynamicClient(scheme, objects...)
}

func TestMetricsCollector_Integration(t *testing.T) {
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "database.example.com/v1alpha1",
				"kind":       "PostgreSQL",
				"metadata": map[string]any{
					"name":      "prod-db",
					"namespace": "production",
					"labels": map[string]any{
						"app.kubernetes.io/name":      "postgresql",
						"app.kubernetes.io/instance":  "prod-db",
						"app.kubernetes.io/component": "database",
					},
				},
				"spec": map[string]any{
					"providerConfigRef": map[string]any{
						"name": "aws-provider",
					},
					"size": "large",
				},
				"status": map[string]any{
					"conditions": []any{
						map[string]any{
							"type":   "Ready",
							"reason": "Available",
							"status": "True",
						},
						map[string]any{
							"type":   "Synced",
							"reason": "ReconcileSuccess",
							"status": "True",
						},
					},
				},
			},
		},
	}

	client := createTestClient(resources...)
	labelMappings := map[string]string{
		"app.kubernetes.io/name":      "app_name",
		"app.kubernetes.io/instance":  "app_instance",
		"app.kubernetes.io/component": "app_component",
	}

	collector := NewMetricsCollector(client, labelMappings, nil, logr.Discard())
	// Manually set resourceAPIs for testing (bypassing auto-discovery)
	collector.xrds = map[string][]string{
		"database.example.com/v1alpha1": {"postgresqls"},
	}

	// Register collector
	registry := prometheus.NewRegistry()
	err := registry.Register(collector)
	require.NoError(t, err)

	// Collect metrics
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, metricFamilies, 1)

	family := metricFamilies[0]
	assert.Equal(t, "crossplane_resource_info", family.GetName())
	assert.Len(t, family.GetMetric(), 1)

	metric := family.GetMetric()[0]
	assert.Equal(t, float64(1), metric.GetGauge().GetValue())

	labels := make(map[string]string)
	for _, label := range metric.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	assert.Equal(t, "database.example.com/v1alpha1", labels["api_version"])
	assert.Equal(t, "postgresqls", labels["kind"])
	assert.Equal(t, "prod-db", labels["name"])
	assert.Equal(t, "postgresql", labels["app_name"])
	assert.Equal(t, "prod-db", labels["app_instance"])
	assert.Equal(t, "database", labels["app_component"])
	assert.Equal(t, "Available", labels["status_ready"])
	assert.Equal(t, "ReconcileSuccess", labels["status_synced"])
	assert.Equal(t, "aws-provider", labels["provider_config_ref"])
}
