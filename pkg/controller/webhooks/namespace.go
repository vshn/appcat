package webhooks

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=delete,path=/validate--v1-namespace,mutating=false,failurePolicy=fail,groups="",resources=namespaces,versions=v1,name=namespace.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.CustomValidator = &NamespaceDeletionProtectionHandler{}

// NamespaceDeletionProtectionHandler
type NamespaceDeletionProtectionHandler struct {
	client client.Client
	log    logr.Logger
}

// SetupNamespaceDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupNamespaceDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithValidator(&NamespaceDeletionProtectionHandler{
			client: mgr.GetClient(),
			log:    mgr.GetLogger().WithName("webhook").WithName("namespace"),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *NamespaceDeletionProtectionHandler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *NamespaceDeletionProtectionHandler) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (p *NamespaceDeletionProtectionHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {

	ns, ok := obj.(client.Object)
	if !ok {
		return nil, fmt.Errorf("object is not valid")
	}

	l := p.log.WithValues("object", ns.GetName(), "namespace", ns.GetNamespace(), "GVK", ns.GetObjectKind().GroupVersionKind().String())

	compInfo, err := checkManagedObject(ctx, ns, p.client, l)
	if err != nil {
		return nil, err
	}

	if compInfo.Exists {
		l.Info("Blocking deletion of namespace")
		return nil, fmt.Errorf("composite %s still exists and has deletion protection enabled, namespace is protected", compInfo.Name)
	}

	l.Info("Allowing deletion of namespace")

	return nil, nil
}
