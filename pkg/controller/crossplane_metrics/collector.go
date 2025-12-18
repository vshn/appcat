package crossplane_metrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// MetricsCollector collects generic metrics about Crossplane resources
type MetricsCollector struct {
	client        dynamic.Interface
	log           logr.Logger
	labelMappings map[string]string
	resourceAPIs  map[string][]string
}

// NewMetricsCollector creates a new MetricsCollector
func NewMetricsCollector(client dynamic.Interface, labelMappings map[string]string, resourceAPIs map[string][]string, log logr.Logger) *MetricsCollector {
	return &MetricsCollector{
		client:        client,
		log:           log,
		labelMappings: labelMappings,
		resourceAPIs:  resourceAPIs,
	}
}

// Describe implements the prometheus.Collector interface
func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	// We use a custom collector, so we send a dummy descriptor
	ch <- prometheus.NewDesc("crossplane_resource_info", "Generic Crossplane Resource Info", nil, nil)
}

// Collect implements the prometheus.Collector interface
func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	c.log.V(1).Info("Collecting Crossplane resource metrics")

	// Iterate over all configured CRDs
	for apiVersion, kinds := range c.resourceAPIs {
		apiParts := strings.Split(apiVersion, "/")
		if len(apiParts) != 2 {
			c.log.Error(fmt.Errorf("invalid API version format: %s", apiVersion), "skipping resource")
			continue
		}

		group := apiParts[0]
		version := apiParts[1]

		for _, kind := range kinds {
			if err := c.collectResourceKind(ch, group, version, kind, apiVersion); err != nil {
				c.log.Error(err, "error collecting resources", "kind", kind)
			}
		}
	}
}

func (c *MetricsCollector) collectResourceKind(
	ch chan<- prometheus.Metric,
	group, version, kind, apiVersion string,
) error {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: kind,
	}

	// Set a timeout for the list operation to avoid indefinite hangs of the collector
	// if the API server is slow or unresponsive.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := c.client.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	for _, item := range list.Items {
		if err := c.collectResource(ch, &item, apiVersion, kind); err != nil {
			c.log.Error(err, "error collecting resource", "name", item.GetName())
		}
	}
	return nil
}

func (c *MetricsCollector) collectResource(
	ch chan<- prometheus.Metric,
	resource *unstructured.Unstructured,
	apiVersion, kind string,
) error {
	labels := make(map[string]string)

	// Always present labels
	labels["api_version"] = apiVersion
	labels["kind"] = kind
	labels["name"] = resource.GetName()

	// Extract status conditions
	status, found, err := unstructured.NestedMap(resource.Object, "status")
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if found {
		conditions, found, err := unstructured.NestedSlice(status, "conditions")
		if err != nil {
			return fmt.Errorf("failed to get conditions: %w", err)
		}

		if found {
			for _, cond := range conditions {
				condMap, ok := cond.(map[string]any)
				if !ok {
					continue
				}

				condType, found, err := unstructured.NestedString(condMap, "type")
				if err != nil || !found {
					continue
				}

				reason, found, err := unstructured.NestedString(condMap, "reason")
				if err != nil || !found {
					continue
				}

				labelKey := fmt.Sprintf("status_%s", strings.ToLower(condType))
				labels[labelKey] = reason
			}
		}
	}

	// Map Kubernetes labels to Prometheus labels
	resourceLabels := resource.GetLabels()
	for k8sLabel, promLabel := range c.labelMappings {
		if value, ok := resourceLabels[k8sLabel]; ok && value != "" {
			labels[promLabel] = value
		}
	}

	// Extract providerConfigRef if present
	spec, found, err := unstructured.NestedMap(resource.Object, "spec")
	if err != nil {
		return fmt.Errorf("failed to get spec: %w", err)
	}

	if found {
		providerConfigRef, found, err := unstructured.NestedMap(spec, "providerConfigRef")
		if err != nil {
			return fmt.Errorf("failed to get providerConfigRef: %w", err)
		}

		if found {
			name, found, err := unstructured.NestedString(providerConfigRef, "name")
			if err != nil {
				return fmt.Errorf("failed to get providerConfigRef name: %w", err)
			}
			if found {
				labels["provider_config_ref"] = name
			}
		}
	}

	// Create metric descriptor
	// Sort label keys to ensure deterministic order
	sortedKeys := make([]string, 0, len(labels))
	for k := range labels {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	labelNames := make([]string, 0, len(labels))
	labelValues := make([]string, 0, len(labels))

	for _, k := range sortedKeys {
		labelNames = append(labelNames, k)
		labelValues = append(labelValues, labels[k])
	}

	desc := prometheus.NewDesc(
		"crossplane_resource_info",
		"Generic Crossplane Resource Info",
		labelNames,
		nil,
	)

	// Create and send metric (info metrics always have value 1)
	metric, err := prometheus.NewConstMetric(
		desc,
		prometheus.GaugeValue,
		1,
		labelValues...,
	)
	if err != nil {
		return fmt.Errorf("failed to create metric: %w", err)
	}

	ch <- metric
	return nil
}
