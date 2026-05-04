package tcpgateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newXListenerSetWithGateway(name, ns, gwName, gwNs string, ports ...int64) *unstructured.Unstructured {
	listeners := make([]any, len(ports))
	for i, p := range ports {
		listeners[i] = map[string]any{
			"name":     "listener",
			"port":     p,
			"protocol": "TCP",
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "gateway.networking.x-k8s.io/v1alpha1",
			"kind":       "XListenerSet",
			"metadata": map[string]any{
				"name":      name,
				"namespace": ns,
			},
			"spec": map[string]any{
				"parentRef": map[string]any{
					"group":     "gateway.networking.k8s.io",
					"kind":      "Gateway",
					"namespace": gwNs,
					"name":      gwName,
				},
				"listeners": listeners,
			},
		},
	}
}

func newTestSharding(gateways []GatewayKey, capacity int) *GatewaySharding {
	return NewGatewaySharding(gateways, capacity)
}

func TestSelectGateway_UnderCapacity_NoReassignment(t *testing.T) {
	gs := newTestSharding([]GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
	}, 10)

	current := GatewayKey{Namespace: "gw-ns", Name: "gw-1"}
	counts := map[GatewayKey]int{current: 5}

	selected, changed, err := gs.SelectGateway(current, 1, counts, nil)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, current, selected)
}

func TestSelectGateway_AtCapacity_Reassign(t *testing.T) {
	gs := newTestSharding([]GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
	}, 10)

	current := GatewayKey{Namespace: "gw-ns", Name: "gw-1"}
	counts := map[GatewayKey]int{current: 10}

	selected, changed, err := gs.SelectGateway(current, 1, counts, nil)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, GatewayKey{Namespace: "gw-ns", Name: "gw-2"}, selected)
}

func TestSelectGateway_AllFull_Error(t *testing.T) {
	gs := newTestSharding([]GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
	}, 2)

	current := GatewayKey{Namespace: "gw-ns", Name: "gw-1"}
	counts := map[GatewayKey]int{
		{Namespace: "gw-ns", Name: "gw-1"}: 2,
		{Namespace: "gw-ns", Name: "gw-2"}: 2,
	}

	_, _, err := gs.SelectGateway(current, 1, counts, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all gateways are full")
}

func TestSelectGateway_MultipleListenersMustFit(t *testing.T) {
	gs := newTestSharding([]GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
	}, 5)

	current := GatewayKey{Namespace: "gw-ns", Name: "gw-1"}
	// gw-1: 4+3=7 > 5 (doesn't fit), gw-2: 2+3=5 <= 5 (fits)
	counts := map[GatewayKey]int{
		current: 4,
		{
			Namespace: "gw-ns",
			Name:      "gw-2",
		}: 2,
	}

	selected, changed, err := gs.SelectGateway(current, 3, counts, nil)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, GatewayKey{Namespace: "gw-ns", Name: "gw-2"}, selected)
}

func TestSelectGateway_LeastListeners(t *testing.T) {
	// gw-3 has fewest listeners (1) but comes last alphabetically.
	// It should still be selected over gw-1 (2 listeners).
	gs := newTestSharding([]GatewayKey{
		{Namespace: "gw-ns", Name: "gw-3"},
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
	}, 5)

	current := GatewayKey{Namespace: "gw-ns", Name: "gw-2"}
	counts := map[GatewayKey]int{
		{Namespace: "gw-ns", Name: "gw-1"}: 2,
		{Namespace: "gw-ns", Name: "gw-2"}: 5, // full
		{Namespace: "gw-ns", Name: "gw-3"}: 1,
	}

	selected, changed, err := gs.SelectGateway(current, 1, counts, nil)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, GatewayKey{Namespace: "gw-ns", Name: "gw-3"}, selected)
}

func TestSelectGateway_UnknownCurrentRef_Reassigns(t *testing.T) {
	gs := newTestSharding([]GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
	}, 10)

	// currentRef points to a gateway not in the configured list.
	unknown := GatewayKey{Namespace: "gw-ns", Name: "gw-unknown"}
	counts := map[GatewayKey]int{
		{Namespace: "gw-ns", Name: "gw-1"}: 3,
		{Namespace: "gw-ns", Name: "gw-2"}: 5,
	}

	selected, changed, err := gs.SelectGateway(unknown, 1, counts, nil)
	require.NoError(t, err)
	assert.True(t, changed, "should reassign away from unknown gateway")
	assert.Equal(t, GatewayKey{Namespace: "gw-ns", Name: "gw-1"}, selected, "should pick least-loaded known gateway")
}

func TestSelectGateway_AllowedGateways_RestrictsScope(t *testing.T) {
	// 3 gateways configured, but only gw-1 and gw-3 are allowed.
	// gw-1 is full, gw-2 has room but isn't allowed, gw-3 has room.
	gs := newTestSharding([]GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
		{Namespace: "gw-ns", Name: "gw-3"},
	}, 5)

	current := GatewayKey{Namespace: "gw-ns", Name: "gw-1"}
	counts := map[GatewayKey]int{
		{Namespace: "gw-ns", Name: "gw-1"}: 5, // full
		{Namespace: "gw-ns", Name: "gw-2"}: 0, // empty but not allowed
		{Namespace: "gw-ns", Name: "gw-3"}: 2,
	}

	allowed := []GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-3"},
	}

	selected, changed, err := gs.SelectGateway(current, 1, counts, allowed)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, GatewayKey{Namespace: "gw-ns", Name: "gw-3"}, selected)
}

func TestSelectGateway_AllowedGateways_AllFull(t *testing.T) {
	// Both allowed gateways full, gw-3 has room but isn't allowed.
	gs := newTestSharding([]GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
		{Namespace: "gw-ns", Name: "gw-3"},
	}, 2)

	current := GatewayKey{Namespace: "gw-ns", Name: "gw-1"}
	counts := map[GatewayKey]int{
		{Namespace: "gw-ns", Name: "gw-1"}: 2,
		{Namespace: "gw-ns", Name: "gw-2"}: 2,
		{Namespace: "gw-ns", Name: "gw-3"}: 0,
	}

	allowed := []GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
	}

	_, _, err := gs.SelectGateway(current, 1, counts, allowed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all gateways are full")
}

func TestCountListenersPerGateway(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XListenerSetList"},
		&unstructured.UnstructuredList{},
	)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		newXListenerSetWithGateway("a", "ns1", "gw-1", "gw-ns", 10000, 10001),
		newXListenerSetWithGateway("b", "ns1", "gw-1", "gw-ns", 10002),
		newXListenerSetWithGateway("c", "ns2", "gw-2", "gw-ns", 10003, 10004, 10005),
	).Build()

	alloc := NewPortAllocator(c, 10000, 29999)

	items, err := alloc.listXListenerSets(context.Background())
	require.NoError(t, err)
	counts := alloc.extractListenerCounts(items)

	assert.Equal(t, 3, counts[GatewayKey{Namespace: "gw-ns", Name: "gw-1"}])
	assert.Equal(t, 3, counts[GatewayKey{Namespace: "gw-ns", Name: "gw-2"}])
}
