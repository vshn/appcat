package sshgateway

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var xListenerSetGVK = schema.GroupVersionKind{
	Group:   "gateway.networking.x-k8s.io",
	Version: "v1alpha1",
	Kind:    "XListenerSetList",
}

// GatewayKey uniquely identifies a Gateway by namespace and name.
type GatewayKey struct {
	Namespace string
	Name      string
}

// PortAllocator allocates unique TCP ports by scanning existing XListenerSet
// resources on the cluster to find used ports, then picking the first free
// port in the configured range.
// Deleted XListenerSets automatically free their ports.
type PortAllocator struct {
	client         client.Client
	portRangeStart int32
	portRangeEnd   int32
}

// NewPortAllocator creates a new PortAllocator.
func NewPortAllocator(c client.Client, portRangeStart, portRangeEnd int32) *PortAllocator {
	return &PortAllocator{
		client:         c,
		portRangeStart: portRangeStart,
		portRangeEnd:   portRangeEnd,
	}
}

// listXListenerSets lists all XListenerSet resources across all namespaces.
func (a *PortAllocator) listXListenerSets(ctx context.Context) ([]unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(xListenerSetGVK)

	if err := a.client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("listing XListenerSets: %w", err)
	}

	return list.Items, nil
}

// AllocatePort finds the first free port in the range given the already used
// ports. The exclude set allows callers to reserve ports that have been
// allocated within the same request but not yet persisted.
func (a *PortAllocator) AllocatePort(usedPorts map[int32]bool, exclude map[int32]bool) (int32, error) {
	for port := a.portRangeStart; port <= a.portRangeEnd; port++ {
		if !usedPorts[port] && !exclude[port] {
			return port, nil
		}
	}

	return 0, fmt.Errorf("port range exhausted: all ports in %d-%d are in use", a.portRangeStart, a.portRangeEnd)
}

// extractUsedPorts collects the ports from XListenerSet listeners.
func (a *PortAllocator) extractUsedPorts(items []unstructured.Unstructured) map[int32]bool {
	usedPorts := make(map[int32]bool)
	for _, item := range items {
		listeners, found, err := unstructured.NestedSlice(item.Object, "spec", "listeners")
		if err != nil || !found {
			continue
		}

		for _, l := range listeners {
			lMap, ok := l.(map[string]any)
			if !ok {
				continue
			}

			port, found, err := unstructured.NestedFieldNoCopy(lMap, "port")
			if err != nil || !found {
				continue
			}

			if p := toInt32(port); p > 0 {
				usedPorts[p] = true
			}
		}
	}

	return usedPorts
}

// extractListenerCounts groups listener counts by their parentRef gateway.
// This is used by the sharding logic to determine how many listeners each
// gateway currently has.
func (a *PortAllocator) extractListenerCounts(items []unstructured.Unstructured) map[GatewayKey]int {
	counts := make(map[GatewayKey]int)
	for _, item := range items {
		gw := extractGatewayKey(item)
		if gw.Name == "" {
			continue
		}

		listeners, found, err := unstructured.NestedSlice(item.Object, "spec", "listeners")
		if err != nil || !found {
			continue
		}

		counts[gw] += len(listeners)
	}

	return counts
}

// extractGatewayKey reads the parentRef from an XListenerSet
// and returns it as a GatewayKey.
func extractGatewayKey(obj unstructured.Unstructured) GatewayKey {
	ns, _, _ := unstructured.NestedString(obj.Object, "spec", "parentRef", "namespace")
	name, _, _ := unstructured.NestedString(obj.Object, "spec", "parentRef", "name")

	return GatewayKey{
		Namespace: ns,
		Name:      name,
	}
}

func toInt32(v any) int32 {
	switch p := v.(type) {
	case int64:
		return int32(p)
	case float64:
		return int32(p)
	default:
		return 0
	}
}
