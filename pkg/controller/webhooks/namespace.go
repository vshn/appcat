package webhooks

import (
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

//+kubebuilder:webhook:verbs=delete,path=/validate--v1-namespace,mutating=false,failurePolicy=fail,groups="",resources=namespaces,versions=v1,name=namespace.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

// SetupNamespaceDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupNamespaceDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {
	cpClient, err := getControlPlaneClient(mgr)
	if err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithValidator(&GenericDeletionProtectionHandler{
			client:             mgr.GetClient(),
			controlPlaneClient: cpClient,
			log:                mgr.GetLogger().WithName("webhook").WithName("namespace"),
		}).
		Complete()
}
