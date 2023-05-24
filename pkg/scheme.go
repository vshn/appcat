package pkg

import (
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	stackgresv1 "github.com/vshn/appcat/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func SetupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.SchemeBuilder.AddToScheme(s)
	_ = xkube.SchemeBuilder.AddToScheme(s)
	_ = vshnv1.SchemeBuilder.SchemeBuilder.AddToScheme(s)
	_ = stackgresv1.SchemeBuilder.AddToScheme(s)
	_ = rbacv1.SchemeBuilder.AddToScheme(s)
	_ = batchv1.SchemeBuilder.AddToScheme(s)
	return s
}
