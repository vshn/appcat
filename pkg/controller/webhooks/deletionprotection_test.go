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
				runtime.OwnerKindAnnotation:    kind,
				runtime.OwnerGroupAnnotation:   vshnv1.GroupVersion.Group,
				runtime.OwnerVersionAnnotation: vshnv1.GroupVersion.Version,
				"crossplane.io/composite":      "redis",
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
	compInfo, err := checkManagedObject(context.TODO(), obj, c, logr.Discard())
	assert.NoError(t, err)
	assert.Equal(t, compositeInfo{Exists: true, Name: "redis"}, compInfo)

	// Given deletion override
	labels := obj.GetLabels()
	labels[ProtectionOverrideLabel] = "true"
	obj.SetLabels(labels)

	// Then don't expect parent
	compInfo, err = checkManagedObject(context.TODO(), obj, c, logr.Discard())
	assert.NoError(t, err)
	assert.Equal(t, compositeInfo{Exists: false, Name: "redis"}, compInfo)
}
