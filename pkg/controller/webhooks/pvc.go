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

var _ webhook.CustomValidator = &PVCDeletionProtectionHandler{}

// PVCDeletionProtectionHandler
type PVCDeletionProtectionHandler struct {
	client client.Client
	log    logr.Logger
}

// SetupPVCDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupPVCDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		WithValidator(&PVCDeletionProtectionHandler{
			client: mgr.GetClient(),
			log:    mgr.GetLogger().WithName("webhook").WithName("pvc"),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *PVCDeletionProtectionHandler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *PVCDeletionProtectionHandler) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (p *PVCDeletionProtectionHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {

	pvc, ok := obj.(client.Object)
	if !ok {
		return nil, fmt.Errorf("object is not valid")
	}

	l := p.log.WithValues("object", pvc.GetName(), "namespace", pvc.GetNamespace(), "kind", pvc.GetObjectKind())

	compInfo, err := checkUnmanagedObject(ctx, pvc, p.client, l)
	if err != nil {
		return nil, err
	}

	if compInfo.Exists {
		return nil, fmt.Errorf("composite %s still exists and has deletion protection enabled, pvc is protected", compInfo.Name)
	}

	return nil, nil
}
