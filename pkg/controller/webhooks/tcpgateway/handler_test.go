package tcpgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func newTestHandler(t *testing.T, existingXLS ...*unstructured.Unstructured) *XListenerSetHandler {
	t.Helper()

	scheme := k8sruntime.NewScheme()
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
	for _, obj := range existingXLS {
		builder = builder.WithObjects(obj)
	}
	c := builder.Build()

	return &XListenerSetHandler{
		allocator: NewPortAllocator(c, 10000, 29999),
		log:       logr.Discard(),
		leaseNS:   "test-ns",
	}
}

func newTestHandlerWithClient(t *testing.T, c client.Client) *XListenerSetHandler {
	t.Helper()

	return &XListenerSetHandler{
		allocator: NewPortAllocator(c, 10000, 29999),
		log:       logr.Discard(),
		leaseNS:   "test-ns",
	}
}

func newTestHandlerWithSharding(t *testing.T, capacity int, xlsSets []*unstructured.Unstructured, gateways []GatewayKey) *XListenerSetHandler {
	t.Helper()

	scheme := k8sruntime.NewScheme()
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
	for _, obj := range xlsSets {
		builder = builder.WithObjects(obj)
	}
	c := builder.Build()

	handler := &XListenerSetHandler{
		allocator: NewPortAllocator(c, 10000, 29999),
		log:       logr.Discard(),
		leaseNS:   "test-ns",
	}
	if capacity > 0 && len(gateways) > 0 {
		handler.sharding = NewGatewaySharding(gateways, capacity)
	}
	return handler
}

func makeAdmissionRequest(t *testing.T, xls *unstructured.Unstructured) admission.Request {
	t.Helper()
	raw, err := json.Marshal(xls)
	require.NoError(t, err)

	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Object: k8sruntime.RawExtension{
				Raw: raw,
			},
		},
	}
}

func TestHandle_PortZeroGetsAllocated(t *testing.T) {
	// Existing XListenerSets use ports 10000-10004
	handler := newTestHandler(t,
		newXListenerSet("existing", "gw-ns", 10000, 10001, 10002, 10003, 10004),
	)

	xls := newXListenerSet("test", "ns", 0)

	raw, err := json.Marshal(xls)
	require.NoError(t, err)

	resp := handler.Handle(context.Background(), makeAdmissionRequest(t, xls))
	assert.True(t, resp.Allowed)
	assert.NotEmpty(t, resp.Patches)

	patched := applyPatches(t, raw, resp)
	listeners, found, err := unstructured.NestedSlice(patched.Object, "spec", "listeners")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, float64(10005), listeners[0].(map[string]any)["port"])
}

func TestHandle_NonZeroPortPassesThrough(t *testing.T) {
	handler := newTestHandler(t)

	xls := newXListenerSet("test", "ns", 22222)

	resp := handler.Handle(context.Background(), makeAdmissionRequest(t, xls))
	assert.True(t, resp.Allowed)
	assert.Empty(t, resp.Patches)
}

func TestHandle_MultipleListeners_OnlyZeroAllocated(t *testing.T) {
	// 10000-10009 are taken
	handler := newTestHandler(t,
		newXListenerSet("a", "ns", 10000, 10001, 10002, 10003, 10004),
		newXListenerSet("b", "ns", 10005, 10006, 10007, 10008, 10009),
	)

	xls := newXListenerSet("test", "ns", 0, 9999, 0)

	raw, err := json.Marshal(xls)
	require.NoError(t, err)

	resp := handler.Handle(context.Background(), makeAdmissionRequest(t, xls))
	assert.True(t, resp.Allowed)
	assert.NotEmpty(t, resp.Patches)

	patched := applyPatches(t, raw, resp)
	listeners, found, err := unstructured.NestedSlice(patched.Object, "spec", "listeners")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, float64(10010), listeners[0].(map[string]any)["port"])
	assert.Equal(t, float64(9999), listeners[1].(map[string]any)["port"])
	assert.Equal(t, float64(10011), listeners[2].(map[string]any)["port"])
}

func TestHandle_NonCreateOperation_Allowed(t *testing.T) {
	handler := newTestHandler(t)

	xls := newXListenerSet("test", "ns", 0)
	raw, err := json.Marshal(xls)
	require.NoError(t, err)

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    k8sruntime.RawExtension{Raw: raw},
		},
	}

	resp := handler.Handle(context.Background(), req)
	assert.True(t, resp.Allowed)
	assert.Empty(t, resp.Patches)
}

func TestHandle_AllowedRoutesPreserved(t *testing.T) {
	handler := newTestHandler(t)

	// Build an XListenerSet with allowedRoutes using raw JSON
	raw := []byte(`{
		"apiVersion": "gateway.networking.x-k8s.io/v1alpha1",
		"kind": "XListenerSet",
		"spec": {
			"parentRef": {
				"name": "tcp-gateway",
				"namespace": "gw-ns"
			},
			"listeners": [{
				"name": "ssh",
				"port": 0,
				"protocol": "TCP",
				"allowedRoutes": {
					"kinds": [{"group": "gateway.networking.k8s.io", "kind": "TCPRoute"}],
					"namespaces": {"from": "All"}
				}
			}]
		}
	}`)

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Object:    k8sruntime.RawExtension{Raw: raw},
		},
	}

	resp := handler.Handle(context.Background(), req)
	assert.True(t, resp.Allowed)
	assert.NotEmpty(t, resp.Patches)

	// Apply patches to verify allowedRoutes survive
	patched := applyPatches(t, raw, resp)
	listeners, found, err := unstructured.NestedSlice(patched.Object, "spec", "listeners")
	require.NoError(t, err)
	require.True(t, found)

	l0 := listeners[0].(map[string]any)
	assert.Equal(t, float64(10000), l0["port"])

	// allowedRoutes must still be present
	allowedRoutes, ok := l0["allowedRoutes"].(map[string]any)
	require.True(t, ok, "allowedRoutes should be preserved")
	kinds := allowedRoutes["kinds"].([]any)
	require.Len(t, kinds, 1)
	assert.Equal(t, "TCPRoute", kinds[0].(map[string]any)["kind"])
	namespaces := allowedRoutes["namespaces"].(map[string]any)
	assert.Equal(t, "All", namespaces["from"])
}

func TestHandle_ShardingReassignsGateway(t *testing.T) {
	// gw-1 already has 10 listeners (at capacity)
	existingXLS := []*unstructured.Unstructured{
		newXListenerSetWithGateway("existing", "ns", "gw-1", "gw-ns", 10000, 10001, 10002, 10003, 10004, 10005, 10006, 10007, 10008, 10009),
	}
	gateways := []GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
	}

	handler := newTestHandlerWithSharding(t, 10, existingXLS, gateways)

	xls := newXListenerSetWithGateway("new", "ns", "gw-1", "gw-ns", 0)

	raw, err := json.Marshal(xls)
	require.NoError(t, err)

	resp := handler.Handle(context.Background(), makeAdmissionRequest(t, xls))
	assert.True(t, resp.Allowed)
	assert.NotEmpty(t, resp.Patches)

	patched := applyPatches(t, raw, resp)

	name, found, err := unstructured.NestedString(patched.Object, "spec", "parentRef", "name")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "gw-2", name)

	ns, found, err := unstructured.NestedString(patched.Object, "spec", "parentRef", "namespace")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "gw-ns", ns)

	listeners, found, err := unstructured.NestedSlice(patched.Object, "spec", "listeners")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, float64(10010), listeners[0].(map[string]any)["port"])
}

func TestHandle_ShardingDisabled_NoReassignment(t *testing.T) {
	// capacity=0 disables sharding, handler should not touch parentRef
	existingXLS := []*unstructured.Unstructured{
		newXListenerSetWithGateway("existing", "ns", "gw-1", "gw-ns", 10000, 10001, 10002, 10003, 10004, 10005, 10006, 10007, 10008, 10009),
	}
	gateways := []GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
	}

	handler := newTestHandlerWithSharding(t, 0, existingXLS, gateways)

	xls := newXListenerSetWithGateway("new", "ns", "gw-1", "gw-ns", 0)

	raw, err := json.Marshal(xls)
	require.NoError(t, err)

	resp := handler.Handle(context.Background(), makeAdmissionRequest(t, xls))
	assert.True(t, resp.Allowed)

	patched := applyPatches(t, raw, resp)

	// parentRef should remain unchanged
	name, found, err := unstructured.NestedString(patched.Object, "spec", "parentRef", "name")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "gw-1", name)

	ns, found, err := unstructured.NestedString(patched.Object, "spec", "parentRef", "namespace")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "gw-ns", ns)
}

func TestHandle_AllGatewaysFull_Denied(t *testing.T) {
	// Both gateways at capacity
	existingXLS := []*unstructured.Unstructured{
		newXListenerSetWithGateway("a", "ns", "gw-1", "gw-ns", 10000, 10001),
		newXListenerSetWithGateway("b", "ns", "gw-2", "gw-ns", 10002, 10003),
	}
	gateways := []GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
		{Namespace: "gw-ns", Name: "gw-2"},
	}

	handler := newTestHandlerWithSharding(t, 2, existingXLS, gateways)

	xls := newXListenerSetWithGateway("new", "ns", "gw-1", "gw-ns", 0)

	resp := handler.Handle(context.Background(), makeAdmissionRequest(t, xls))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "all gateways are full")
}

func TestHandle_MultipleReplicas_UniquePorts(t *testing.T) {
	// simulates shared etcd across replicas.
	scheme := k8sruntime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XListenerSetList"},
		&unstructured.UnstructuredList{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XListenerSet"},
		&unstructured.Unstructured{},
	)
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	existing := newXListenerSet("existing", "gw-ns", 10000, 10001, 10002)
	sharedClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	const numReplicas = 5

	ports := make([]float64, numReplicas)
	for i := range numReplicas {
		handler := newTestHandlerWithClient(t, sharedClient)
		xls := newXListenerSet(fmt.Sprintf("test-%d", i), "ns", 0)
		raw, _ := json.Marshal(xls)

		resp := handler.Handle(context.Background(), makeAdmissionRequest(t, xls))
		require.True(t, resp.Allowed)
		require.NotEmpty(t, resp.Patches)

		patched := applyPatches(t, raw, resp)
		listeners, _, _ := unstructured.NestedSlice(patched.Object, "spec", "listeners")
		ports[i] = listeners[0].(map[string]any)["port"].(float64)
	}

	seen := make(map[float64]bool)
	for _, p := range ports {
		seen[p] = true
	}

	t.Logf("Allocated ports: %v", ports)
	assert.Equal(t, numReplicas, len(seen),
		"all replicas should allocate unique ports")

	for i, expected := range []float64{10003, 10004, 10005, 10006, 10007} {
		assert.Equal(t, expected, ports[i], "replica %d should get port %v", i, expected)
	}
}

func TestHandle_ShardingMissingParentRef_Denied(t *testing.T) {
	gateways := []GatewayKey{
		{Namespace: "gw-ns", Name: "gw-1"},
	}

	handler := newTestHandlerWithSharding(t, 10, nil, gateways)

	// XListenerSet without parentRef name
	xls := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "gateway.networking.x-k8s.io/v1alpha1",
			"kind":       "XListenerSet",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "ns",
			},
			"spec": map[string]any{
				"parentRef": map[string]any{
					"namespace": "gw-ns",
				},
				"listeners": []any{
					map[string]any{
						"name":     "ssh",
						"port":     float64(0),
						"protocol": "TCP",
					},
				},
			},
		},
	}

	resp := handler.Handle(context.Background(), makeAdmissionRequest(t, xls))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "spec.parentRef.name is required")
}

// applyPatches applies JSON patches from the admission response to raw bytes
// and returns the result as an Unstructured object.
func applyPatches(t *testing.T, raw []byte, resp admission.Response) *unstructured.Unstructured {
	t.Helper()
	patched := applyRawPatches(t, raw, resp)
	obj := &unstructured.Unstructured{}
	require.NoError(t, json.Unmarshal(patched, &obj.Object))
	return obj
}

// applyRawPatches applies JSON patches to raw bytes and returns the patched bytes.
func applyRawPatches(t *testing.T, raw []byte, resp admission.Response) []byte {
	t.Helper()

	var obj map[string]any
	require.NoError(t, json.Unmarshal(raw, &obj))

	for _, p := range resp.Patches {
		if p.Operation == "replace" {
			setNestedValue(obj, p.Path, p.Value)
		}
	}

	result, err := json.Marshal(obj)
	require.NoError(t, err)
	return result
}

// setNestedValue sets a value at a JSON pointer path in a nested map.
func setNestedValue(obj map[string]any, path string, value any) {
	// Parse JSON pointer path like /spec/listeners/0/port
	parts := splitJSONPointer(path)
	if len(parts) == 0 {
		return
	}

	var current any = obj
	for _, part := range parts[:len(parts)-1] {
		switch c := current.(type) {
		case map[string]any:
			current = c[part]
		case []any:
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err == nil && idx < len(c) {
				current = c[idx]
			} else {
				return
			}
		default:
			return
		}
	}

	lastPart := parts[len(parts)-1]
	switch c := current.(type) {
	case map[string]any:
		c[lastPart] = value
	case []any:
		var idx int
		if _, err := fmt.Sscanf(lastPart, "%d", &idx); err == nil && idx < len(c) {
			c[idx] = value
		}
	}
}

func splitJSONPointer(path string) []string {
	if path == "" || path == "/" {
		return nil
	}
	// Remove leading /
	if path[0] == '/' {
		path = path[1:]
	}
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	parts = append(parts, path[start:])
	return parts
}
