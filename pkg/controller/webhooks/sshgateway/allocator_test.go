package sshgateway

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XListenerSet"},
		&unstructured.Unstructured{},
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

func newTestAllocatorWithClient(t *testing.T, c client.Client) *PortAllocator {
	t.Helper()
	return NewPortAllocator(c, 10000, 29999)
}

func newTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XListenerSetList"},
		&unstructured.UnstructuredList{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XListenerSet"},
		&unstructured.Unstructured{},
	)
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestTryReclaimStaleLease_HolderGone(t *testing.T) {
	holderName := "gone-xls"
	oldTime := metav1.NewMicroTime(time.Now().Add(-5 * time.Minute))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("ssh-port-%d", 10000),
			Namespace: "test-ns",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &holderName,
			AcquireTime:    &oldTime,
		},
	}

	c := newTestClient(t, lease)
	alloc := newTestAllocatorWithClient(t, c)

	reclaimed := alloc.tryReclaimStaleLease(context.Background(), lease.Name, "test-ns")
	assert.True(t, reclaimed, "should reclaim lease when holder XListenerSet is gone")

	err := c.Get(context.Background(), client.ObjectKeyFromObject(lease), &coordinationv1.Lease{})
	assert.True(t, apierrors.IsNotFound(err), "lease should be deleted")
}

func TestTryReclaimStaleLease_HolderExists(t *testing.T) {
	holderName := "existing-xls"
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("ssh-port-%d", 10000),
			Namespace: "test-ns",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &holderName,
		},
	}

	xls := newXListenerSet("existing-xls", "test-ns", 10000)
	c := newTestClient(t, lease, xls)
	alloc := newTestAllocatorWithClient(t, c)

	reclaimed := alloc.tryReclaimStaleLease(context.Background(), lease.Name, "test-ns")
	assert.False(t, reclaimed, "should not reclaim lease when holder XListenerSet still exists")
}

func TestTryReclaimStaleLease_NoHolderIdentity(t *testing.T) {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("ssh-port-%d", 10000),
			Namespace: "test-ns",
		},
		Spec: coordinationv1.LeaseSpec{},
	}

	c := newTestClient(t, lease)
	alloc := newTestAllocatorWithClient(t, c)

	reclaimed := alloc.tryReclaimStaleLease(context.Background(), lease.Name, "test-ns")
	assert.True(t, reclaimed, "should reclaim lease with no holder identity")

	err := c.Get(context.Background(), client.ObjectKeyFromObject(lease), &coordinationv1.Lease{})
	assert.True(t, apierrors.IsNotFound(err), "lease should be deleted")
}

func TestTryReclaimStaleLease_HolderGoneButRecent(t *testing.T) {
	holderName := "pending-xls"
	now := metav1.NewMicroTime(time.Now())
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("ssh-port-%d", 10000),
			Namespace: "test-ns",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &holderName,
			AcquireTime:    &now,
		},
	}

	c := newTestClient(t, lease)
	alloc := newTestAllocatorWithClient(t, c)

	reclaimed := alloc.tryReclaimStaleLease(context.Background(), lease.Name, "test-ns")
	assert.False(t, reclaimed, "should not reclaim recent lease even when holder is gone")
}
