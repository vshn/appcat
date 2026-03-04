package sshgateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newXListenerSet(name, namespace string, ports ...int64) *unstructured.Unstructured {
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
				"namespace": namespace,
			},
			"spec": map[string]any{
				"listeners": listeners,
			},
		},
	}
}

func newTestAllocator(t *testing.T, objects ...*unstructured.Unstructured) *PortAllocator {
	t.Helper()

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XListenerSetList"},
		&unstructured.UnstructuredList{},
	)
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, obj := range objects {
		builder = builder.WithObjects(obj)
	}
	c := builder.Build()

	return NewPortAllocator(c, 10000, 29999)
}

func TestAllocatePort_EmptyCluster(t *testing.T) {
	alloc := newTestAllocator(t)

	items, err := alloc.listXListenerSets(context.Background())
	require.NoError(t, err)
	usedPorts := alloc.extractUsedPorts(items)
	port, err := alloc.AllocatePort(context.Background(), usedPorts, "test-ns", "holder")
	assert.NoError(t, err)
	assert.Equal(t, int32(10000), port)
}

func TestAllocatePort_SkipsUsedPorts(t *testing.T) {
	alloc := newTestAllocator(t,
		newXListenerSet("a-ssh", "gw-ns", 10000),
		newXListenerSet("b-ssh", "gw-ns", 10001),
		newXListenerSet("c-ssh", "gw-ns", 10003),
	)

	items, err := alloc.listXListenerSets(context.Background())
	require.NoError(t, err)
	usedPorts := alloc.extractUsedPorts(items)
	port, err := alloc.AllocatePort(context.Background(), usedPorts, "test-ns", "holder")
	assert.NoError(t, err)
	// 10000, 10001 are taken; 10002 is the first free
	assert.Equal(t, int32(10002), port)
}

func TestAllocatePort_RangeExhausted(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XListenerSetList"},
		&unstructured.UnstructuredList{},
	)
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	// Fill range 20000-20002 (3 ports)
	builder = builder.WithObjects(
		newXListenerSet("a", "ns", 20000),
		newXListenerSet("b", "ns", 20001),
		newXListenerSet("c", "ns", 20002),
	)
	c := builder.Build()

	alloc := NewPortAllocator(c, 20000, 20002)

	items, err := alloc.listXListenerSets(context.Background())
	require.NoError(t, err)
	usedPorts := alloc.extractUsedPorts(items)
	_, err = alloc.AllocatePort(context.Background(), usedPorts, "test-ns", "holder")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port range exhausted")
}

func TestAllocatePort_IgnoresZeroPorts(t *testing.T) {
	// An XListenerSet with port 0 (not yet allocated by webhook) should not
	// count as a used port.
	alloc := newTestAllocator(t,
		newXListenerSet("pending", "gw-ns", 0),
	)

	items, err := alloc.listXListenerSets(context.Background())
	require.NoError(t, err)
	usedPorts := alloc.extractUsedPorts(items)
	port, err := alloc.AllocatePort(context.Background(), usedPorts, "test-ns", "holder")
	assert.NoError(t, err)
	assert.Equal(t, int32(10000), port)
}

func TestAllocatePort_MultipleListenersPerXLS(t *testing.T) {
	alloc := newTestAllocator(t,
		newXListenerSet("multi", "gw-ns", 10000, 10001),
	)

	items, err := alloc.listXListenerSets(context.Background())
	require.NoError(t, err)
	usedPorts := alloc.extractUsedPorts(items)
	port, err := alloc.AllocatePort(context.Background(), usedPorts, "test-ns", "holder")
	assert.NoError(t, err)
	assert.Equal(t, int32(10002), port)
}
