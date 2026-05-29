package compat

import (
	"context"
	"encoding/json"
	"testing"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

// newTestRuntime builds a ServiceRuntime with a minimal observed composite and,
// when matrixCM is non-nil, the matrix ConfigMap injected as the observed
// provider-kubernetes Object under MatrixObserverName (mirroring how
// GetObservedKubeObject reads Status.AtProvider.Manifest).
func newTestRuntime(t *testing.T, matrixCM *corev1.ConfigMap) *runtime.ServiceRuntime {
	t.Helper()

	composite, err := structpb.NewStruct(map[string]any{
		"apiVersion": "vshn.appcat.vshn.io/v1",
		"kind":       "XVSHNPostgreSQL",
		"metadata":   map[string]any{"name": "test"},
	})
	require.NoError(t, err)

	req := &xfnproto.RunFunctionRequest{
		Observed: &xfnproto.State{
			Composite: &xfnproto.Resource{Resource: composite},
			Resources: map[string]*xfnproto.Resource{},
		},
	}

	if matrixCM != nil {
		raw, err := json.Marshal(matrixCM)
		require.NoError(t, err)

		ko := &xkube.Object{}
		ko.Status.AtProvider.Manifest = k8sruntime.RawExtension{Raw: raw}

		u, err := composed.From(ko)
		require.NoError(t, err)
		s, err := structpb.NewStruct(u.UnstructuredContent())
		require.NoError(t, err)

		req.Observed.Resources[MatrixObserverName] = &xfnproto.Resource{Resource: s}
	}

	svc, err := runtime.NewServiceRuntime(logr.Discard(), corev1.ConfigMap{}, req)
	require.NoError(t, err)
	return svc
}

func newTestRuntimeWithObservedMatrix(t *testing.T, cm *corev1.ConfigMap) *runtime.ServiceRuntime {
	return newTestRuntime(t, cm)
}

func newTestRuntimeWithoutMatrix(t *testing.T) *runtime.ServiceRuntime {
	return newTestRuntime(t, nil)
}

func TestRunCompatCheck_incompatible_setsCondition(t *testing.T) {
	matrixCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: MatrixConfigMapName, Namespace: MatrixNamespace},
		Data: map[string]string{
			"matrix.yaml": "postgresql:\n  - versionRange: \"14.x\"\n    compatibleRevisions: \"<v3.72.0\"\n",
		},
	}
	svc := newTestRuntimeWithObservedMatrix(t, matrixCM)

	var got *vshnv1.Condition
	res := RunCompatCheck(context.Background(), svc, "postgresql", "14", "v3.73.0-v4.190.0",
		func(c vshnv1.Condition) { got = &c })

	assert.Nil(t, res) // no fatal
	require.NotNil(t, got)
	assert.Equal(t, ConditionTypeVersionIncompatible, got.Type)
	assert.Equal(t, metav1.ConditionTrue, got.Status)
}

func TestRunCompatCheck_matrixNotObservedYet_returnsNil(t *testing.T) {
	svc := newTestRuntimeWithoutMatrix(t)
	res := RunCompatCheck(context.Background(), svc, "postgresql", "14", "v3.73.0-v4.190.0",
		func(c vshnv1.Condition) {})
	assert.Nil(t, res)
}

func TestUpsertCondition(t *testing.T) {
	got := UpsertCondition(nil, vshnv1.Condition{Type: "A", Status: metav1.ConditionTrue})
	assert.Len(t, got, 1)
	got = UpsertCondition(got, vshnv1.Condition{Type: "A", Status: metav1.ConditionFalse})
	assert.Len(t, got, 1)
	assert.Equal(t, metav1.ConditionFalse, got[0].Status)
}

func TestRunCompatCheck_emptyRevision_noop(t *testing.T) {
	called := false
	res := RunCompatCheck(context.Background(), &runtime.ServiceRuntime{}, "postgresql", "16", "",
		func(c vshnv1.Condition) { called = true })
	assert.Nil(t, res)
	assert.False(t, called, "no condition should be set when the instance is unpinned")
}

func TestRunCompatCheck_compatible_setsFalseCondition(t *testing.T) {
	matrixCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: MatrixConfigMapName, Namespace: MatrixNamespace},
		Data: map[string]string{
			"matrix.yaml": "postgresql:\n  - versionRange: \"16.x\"\n    compatibleRevisions: \">=v3.70.0\"\n",
		},
	}
	svc := newTestRuntimeWithObservedMatrix(t, matrixCM)
	var got *vshnv1.Condition
	res := RunCompatCheck(context.Background(), svc, "postgresql", "16", "v3.72.2-v4.186.2",
		func(c vshnv1.Condition) { got = &c })
	assert.Nil(t, res)
	require.NotNil(t, got)
	assert.Equal(t, metav1.ConditionFalse, got.Status)
}
