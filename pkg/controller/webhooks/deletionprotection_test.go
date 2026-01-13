package webhooks

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_checkManagedObject(t *testing.T) {

	kind := "XVSHNRedis"
	apiVersion := appcatv1.GroupVersion.Group + "/" + appcatv1.GroupVersion.Version

	typeMeta := metav1.TypeMeta{
		Kind:       kind,
		APIVersion: apiVersion,
	}

	// Given object with owner annotation
	obj := &appcatv1.XObjectBucket{
		TypeMeta: typeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name: "leaf",
			Labels: map[string]string{
				runtime.OwnerKindLabel:      kind,
				runtime.OwnerGroupLabel:     vshnv1.GroupVersion.Group,
				runtime.OwnerVersionLabel:   vshnv1.GroupVersion.Version,
				runtime.OwnerCompositeLabel: "redis",
			},
		},
	}

	// When there are parents
	parents := []client.Object{
		&vshnv1.XVSHNRedis{
			TypeMeta: metav1.TypeMeta{
				Kind:       "XVSHNRedis",
				APIVersion: vshnv1.GroupVersion.Group + "/" + vshnv1.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "redis",
			},
			Spec: vshnv1.XVSHNRedisSpec{},
		},
		&appcatv1.XObjectBucket{
			TypeMeta: typeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name: "bucket1",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: vshnv1.GroupVersion.Group + "/" + vshnv1.GroupVersion.Version,
						Name:       "redis",
						Kind:       "XVSHNRedis",
					},
				},
			},
		},
		&appcatv1.XObjectBucket{
			TypeMeta: typeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name: "bucket2",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: apiVersion,
						Name:       "bucket1",
						Kind:       kind,
					},
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(parents...).
		Build()

	// Then expect parent
	compInfo, err := checkManagedObject(context.TODO(), obj, c, c, logr.Discard())
	assert.NoError(t, err)
	assert.Equal(t, compositeInfo{Exists: true, Name: "redis"}, compInfo)

	// Given deletion override
	labels := obj.GetLabels()
	labels[ProtectionOverrideLabel] = "true"
	obj.SetLabels(labels)

	// Then don't expect parent
	compInfo, err = checkManagedObject(context.TODO(), obj, c, c, logr.Discard())
	assert.NoError(t, err)
	assert.Equal(t, compositeInfo{Exists: false, Name: "redis"}, compInfo)

	// Given unmanaged object
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				runtime.OwnerKindLabel:      kind,
				runtime.OwnerGroupLabel:     vshnv1.GroupVersion.Group,
				runtime.OwnerVersionLabel:   vshnv1.GroupVersion.Version,
				runtime.OwnerCompositeLabel: "redis",
			},
		},
	}

	err = c.Create(context.TODO(), ns)
	assert.NoError(t, err)

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mypvc",
			Namespace: "test",
		},
	}

	err = c.Create(context.TODO(), pvc)
	assert.NoError(t, err)

	// Then expect parent
	compInfo, err = checkUnmanagedObject(context.TODO(), pvc, c, c, logr.Discard())
	assert.NoError(t, err)
	assert.Equal(t, compositeInfo{Exists: true, Name: "redis"}, compInfo)

	// Given override
	labels = map[string]string{}
	labels[ProtectionOverrideLabel] = "true"
	pvc.SetLabels(labels)

	// Then expect no parent
	compInfo, err = checkUnmanagedObject(context.TODO(), pvc, c, c, logr.Discard())
	assert.NoError(t, err)
	assert.Equal(t, compositeInfo{Exists: false, Name: "redis"}, compInfo)

}

func Test_unmanagedPvcDeletion(t *testing.T) {
	const (
		kind      = "XVSHNRedis"
		composite = "redis"
	)
	labels := map[string]string{
		runtime.OwnerKindLabel:      kind,
		runtime.OwnerGroupLabel:     vshnv1.GroupVersion.Group,
		runtime.OwnerVersionLabel:   vshnv1.GroupVersion.Version,
		runtime.OwnerCompositeLabel: composite,
	}

	parents := []client.Object{
		&vshnv1.XVSHNRedis{
			TypeMeta: metav1.TypeMeta{
				Kind:       kind,
				APIVersion: vshnv1.GroupVersion.Group + "/" + vshnv1.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: composite,
			},
			Spec: vshnv1.XVSHNRedisSpec{},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(parents...).
		Build()

	// Given an unmanaged PVC
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind: "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mypvc",
			Namespace: "test",
		},
	}

	err := c.Create(context.TODO(), pvc)
	assert.NoError(t, err)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test",
			Labels: labels,
		},
	}

	err = c.Create(context.TODO(), ns)
	assert.NoError(t, err)

	// Expect TRUE compInfo.Exists given no namespace level override
	compInfo, err := checkUnmanagedObject(context.TODO(), pvc, c, c, logr.Discard())
	assert.NoError(t, err)
	assert.True(t, compInfo.Exists)

	// Expect FALSE compInfo.Exists given namespace level override
	labels[ProtectionOverrideLabelStorage] = "true"
	err = c.Update(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test",
			Labels: labels,
		},
	})
	assert.NoError(t, err)

	compInfo, err = checkUnmanagedObject(context.TODO(), pvc, c, c, logr.Discard())
	assert.NoError(t, err)

	t.Logf("compinfo 2: %v", compInfo)
	assert.False(t, compInfo.Exists)
}
