package webhooks

import (
	xkubev1alpha1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha1"
	xkubev1alpha2 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	ctrl "sigs.k8s.io/controller-runtime"
)

//+kubebuilder:webhook:verbs=delete,path=/validate-kubernetes-crossplane-io-v1alpha2-object,mutating=false,failurePolicy=fail,groups="kubernetes.crossplane.io",resources=objects,versions=v1alpha2,name=objects.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=syn.tools,resources=compositeredisinstances,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=syn.tools,resources=compositeredisinstances/status,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=syn.tools,resources=compositemariadbinstances,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=syn.tools,resources=compositemariadbinstances/status,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=kubernetes.crossplane.io,resources=providerconfigs,verbs=get;list;watch;

// SetupObjectDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupObjectDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&xkubev1alpha2.Object{}).
		WithValidator(&GenericDeletionProtectionHandler{
			client:             mgr.GetClient(),
			controlPlaneClient: mgr.GetClient(),
			log:                mgr.GetLogger().WithName("webhook").WithName("object"),
		}).
		Complete()
}

// SetupObjectv1alpha1DeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupObjectv1alpha1DeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&xkubev1alpha1.Object{}).
		WithValidator(&GenericDeletionProtectionHandler{
			client:             mgr.GetClient(),
			controlPlaneClient: mgr.GetClient(),
			log:                mgr.GetLogger().WithName("webhook").WithName("object"),
		}).
		Complete()
}
