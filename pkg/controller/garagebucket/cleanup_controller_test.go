package garagebucket

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	require.NoError(t, vshnv1.AddToScheme(s))
	return s
}

// garageOperatorFinalizer simulates the finalizer the garage operator sets on
// GarageBucket resources, which keeps the object alive while deletion is pending.
const garageOperatorFinalizer = "garagebucket.garage.rajsingh.info/finalizer"

func newGarageBucket(name, namespace, clusterRefName string, deletionTimestamp *metav1.Time) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(garageBucketGVK)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetResourceVersion("1")
	_ = unstructured.SetNestedField(obj.Object, clusterRefName, "spec", "clusterRef", "name")
	if deletionTimestamp != nil {
		ts := metav1.NewTime(deletionTimestamp.UTC())
		obj.SetDeletionTimestamp(&ts)
		// Kubernetes requires at least one finalizer when deletionTimestamp is set.
		obj.SetFinalizers([]string{garageOperatorFinalizer})
	}
	return obj
}

func newVSHNGarage(name, namespace, instanceNamespace string) *vshnv1.VSHNGarage {
	return &vshnv1.VSHNGarage{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: "1",
		},
		Status: vshnv1.VSHNGarageStatus{
			InstanceNamespace: instanceNamespace,
		},
	}
}

func newAdminSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            adminCredentialsSecret,
			Namespace:       namespace,
			ResourceVersion: "1",
		},
		Data: map[string][]byte{
			"endpoint":          []byte("http://s3.garage.svc:3900"),
			"access-key-id":     []byte("admin-key"),
			"secret-access-key": []byte("admin-secret"),
		},
	}
}

func newReconciler(c client.Client, emptyFn func(context.Context, *corev1.Secret, string, logr.Logger) error) *CleanupReconciler {
	r := NewCleanupReconciler(c, logr.Discard())
	if emptyFn != nil {
		r.emptyBucketFn = emptyFn
	}
	return r
}

func reconcile(t *testing.T, r *CleanupReconciler, namespace, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: namespace, Name: name},
	})
	require.NoError(t, err)
	return res
}

const (
	testBucket     = "my-bucket"
	testGarageNS   = "garage-ns"
	testInstanceNS = "vshn-garage-mygarage-abc12"
	testGarageName = "mygarage"
)

func TestEmptyBucket_MissingOrInvalidCredentials(t *testing.T) {
	r := NewCleanupReconciler(fake.NewClientBuilder().Build(), logr.Discard())

	secretWith := func(data map[string][]byte) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: adminCredentialsSecret, Namespace: testInstanceNS},
			Data:       data,
		}
	}

	fullData := map[string][]byte{
		"endpoint":          []byte("http://s3.garage.svc:3900"),
		"access-key-id":     []byte("admin-key"),
		"secret-access-key": []byte("admin-secret"),
	}

	tests := []struct {
		name        string
		data        map[string][]byte
		errContains string
	}{
		{
			name:        "missing endpoint",
			data:        map[string][]byte{"access-key-id": fullData["access-key-id"], "secret-access-key": fullData["secret-access-key"]},
			errContains: `missing required key "endpoint"`,
		},
		{
			name:        "empty endpoint",
			data:        map[string][]byte{"endpoint": []byte(""), "access-key-id": fullData["access-key-id"], "secret-access-key": fullData["secret-access-key"]},
			errContains: `missing required key "endpoint"`,
		},
		{
			name:        "missing access-key-id",
			data:        map[string][]byte{"endpoint": fullData["endpoint"], "secret-access-key": fullData["secret-access-key"]},
			errContains: `missing required key "access-key-id"`,
		},
		{
			name:        "missing secret-access-key",
			data:        map[string][]byte{"endpoint": fullData["endpoint"], "access-key-id": fullData["access-key-id"]},
			errContains: `missing required key "secret-access-key"`,
		},
		{
			name:        "endpoint without scheme yields empty host",
			data:        map[string][]byte{"endpoint": []byte("s3.garage.svc:3900"), "access-key-id": fullData["access-key-id"], "secret-access-key": fullData["secret-access-key"]},
			errContains: "missing host",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := r.emptyBucket(context.Background(), secretWith(tc.data), testBucket, logr.Discard())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestReconcile_NoDeletionTimestamp_SkipsBucket(t *testing.T) {
	s := newTestScheme(t)
	gb := newGarageBucket(testBucket, testGarageNS, testGarageName, nil) // no DeletionTimestamp
	garage := newVSHNGarage(testGarageName, testGarageNS, testInstanceNS)
	adminSecret := newAdminSecret(testInstanceNS)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(gb, garage, adminSecret).Build()

	emptyCalled := false
	r := newReconciler(c, func(_ context.Context, _ *corev1.Secret, _ string, _ logr.Logger) error {
		emptyCalled = true
		return nil
	})

	reconcile(t, r, testGarageNS, testBucket)

	assert.False(t, emptyCalled, "emptyBucketFn must not be called when DeletionTimestamp is absent")
}

// TestReconcile_AlreadyDeletingOnStartup verifies that a GarageBucket which was
// already marked for deletion before the controller started (no Create/Update
// event fires) is still processed when the reconciler is called directly — as
// happens during the initial list-watch cache sync.
func TestReconcile_AlreadyDeletingOnStartup(t *testing.T) {
	s := newTestScheme(t)
	past := metav1.NewTime(metav1.Now().Add(-5 * time.Minute))
	gb := newGarageBucket(testBucket, testGarageNS, testGarageName, &past)
	garage := newVSHNGarage(testGarageName, testGarageNS, testInstanceNS)
	adminSecret := newAdminSecret(testInstanceNS)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(gb, garage, adminSecret).Build()

	emptyCalled := false
	r := newReconciler(c, func(_ context.Context, _ *corev1.Secret, bucketName string, _ logr.Logger) error {
		emptyCalled = true
		assert.Equal(t, testBucket, bucketName)
		return nil
	})

	reconcile(t, r, testGarageNS, testBucket)

	assert.True(t, emptyCalled, "emptyBucketFn should be called for a pre-existing deleting bucket")
}

func TestReconcile_DeletionEmptiesBucket(t *testing.T) {
	s := newTestScheme(t)
	now := metav1.Now()
	gb := newGarageBucket(testBucket, testGarageNS, testGarageName, &now)
	garage := newVSHNGarage(testGarageName, testGarageNS, testInstanceNS)
	adminSecret := newAdminSecret(testInstanceNS)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(gb, garage, adminSecret).Build()

	emptyCalled := false
	r := newReconciler(c, func(_ context.Context, secret *corev1.Secret, bucketName string, _ logr.Logger) error {
		emptyCalled = true
		assert.Equal(t, testBucket, bucketName)
		assert.Equal(t, adminCredentialsSecret, secret.Name)
		assert.Equal(t, testInstanceNS, secret.Namespace)
		return nil
	})

	reconcile(t, r, testGarageNS, testBucket)

	assert.True(t, emptyCalled, "emptyBucketFn should have been called")
}
