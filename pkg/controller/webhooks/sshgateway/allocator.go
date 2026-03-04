package sshgateway

import (
	"context"
	"fmt"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// AllocatePort atomically reserves a port by creating a Lease.
// If the Lease already exists, it tries the next port.
func (a *PortAllocator) AllocatePort(ctx context.Context, usedPorts map[int32]bool, namespace, holder string) (int32, error) {
	now := metav1.NewMicroTime(time.Now())
	duration := int32(300)

	for port := a.portRangeStart; port <= a.portRangeEnd; port++ {
		if usedPorts[port] {
			continue
		}

		lease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("ssh-port-%d", port),
				Namespace: namespace,
				Labels:    map[string]string{"app.kubernetes.io/managed-by": "sshgateway-port-allocator"},
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &holder,
				LeaseDurationSeconds: &duration,
				AcquireTime:          &now,
			},
		}

		err := a.client.Create(ctx, lease)
		if err == nil {
			return port, nil
		}
		if apierrors.IsAlreadyExists(err) {
			if a.isStaleLease(ctx, lease.Name, namespace, usedPorts[port]) {
				_ = a.client.Delete(ctx, lease)

				if err := a.client.Create(ctx, lease); err == nil {
					return port, nil
				}
			}
			continue
		}
		return 0, fmt.Errorf("reserving port %d: %w", port, err)
	}
	return 0, fmt.Errorf("port range exhausted: all ports in %d-%d are in use", a.portRangeStart, a.portRangeEnd)
}

// isStaleLease checks if an existing Lease is stale and can be deleted to free up a port.
func (a *PortAllocator) isStaleLease(ctx context.Context, name, namespace string, portInUse bool) bool {
	if portInUse {
		return false
	}

	existing := &coordinationv1.Lease{}
	if err := a.client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, existing); err != nil {
		return false
	}

	if existing.Spec.AcquireTime == nil || existing.Spec.LeaseDurationSeconds == nil {
		return true
	}

	expiry := existing.Spec.AcquireTime.Add(time.Duration(*existing.Spec.LeaseDurationSeconds) * time.Second)
	return time.Now().After(expiry)
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
