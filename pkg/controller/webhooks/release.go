package webhooks

import (
	helmv1beta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

//+kubebuilder:webhook:verbs=delete,path=/validate-helm-crossplane-io-v1beta1-release,mutating=false,failurePolicy=fail,groups="helm.crossplane.io",resources=releases,versions=v1beta1,name=releases.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=syn.tools,resources=compositeredisinstances,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=syn.tools,resources=compositeredisinstances/status,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=syn.tools,resources=compositemariadbinstances,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=syn.tools,resources=compositemariadbinstances/status,verbs=get;list;watch;patch;update

// SetupReleaseDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupReleaseDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&helmv1beta1.Release{}).
		WithValidator(&GenericDeletionProtectionHandler{
			client:             mgr.GetClient(),
			controlPlaneClient: mgr.GetClient(),
			log:                mgr.GetLogger().WithName("webhook").WithName("release"),
		}).
		Complete()
}
