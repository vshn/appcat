package pkg

import (
	xhelm "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	miniov1 "github.com/vshn/appcat/v4/apis/minio/v1"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	minioproviderv1 "github.com/vshn/provider-minio/apis/provider/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
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
	_ = miniov1.SchemeBuilder.AddToScheme(s)
	_ = minioproviderv1.SchemeBuilder.AddToScheme(s)
	_ = promv1.AddToScheme(s)
	_ = alertmanagerv1alpha1.AddToScheme(s)
	_ = cmv1.SchemeBuilder.AddToScheme(s)
	_ = netv1.AddToScheme(s)
}
