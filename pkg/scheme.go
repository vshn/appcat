package pkg

import (
	xhelm "github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	stackgresv1 "github.com/vshn/appcat/apis/stackgres/v1"
	appcatv1 "github.com/vshn/appcat/apis/v1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func SetupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	AddToScheme(s)
	return s
}

func AddToScheme(s *runtime.Scheme) {
	_ = corev1.SchemeBuilder.AddToScheme(s)
	_ = xkube.SchemeBuilder.AddToScheme(s)
	_ = vshnv1.SchemeBuilder.SchemeBuilder.AddToScheme(s)
	_ = stackgresv1.SchemeBuilder.AddToScheme(s)
	_ = rbacv1.SchemeBuilder.AddToScheme(s)
	_ = appcatv1.SchemeBuilder.AddToScheme(s)
	_ = batchv1.SchemeBuilder.AddToScheme(s)
	_ = k8upv1.SchemeBuilder.AddToScheme(s)
	_ = xhelm.SchemeBuilder.AddToScheme(s)
	_ = appsv1.SchemeBuilder.AddToScheme(s)
}
