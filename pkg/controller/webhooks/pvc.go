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

//+kubebuilder:webhook:verbs=delete,path=/validate--v1-persistentvolumeclaim,mutating=false,failurePolicy=fail,groups="",resources=persistentvolumeclaims,versions=v1,name=pvc.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.CustomValidator = &PVCDeletionProtectionHandler{}

// PVCDeletionProtectionHandler
type PVCDeletionProtectionHandler struct {
	client             client.Client
	controlPlaneClient client.Client
	log                logr.Logger
}

// SetupPVCDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupPVCDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {
	cpClient, err := getControlPlaneClient(mgr)
	if err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		WithValidator(&PVCDeletionProtectionHandler{
			client:             mgr.GetClient(),
			controlPlaneClient: cpClient,
			log:                mgr.GetLogger().WithName("webhook").WithName("pvc"),
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

	l := p.log.WithValues("object", pvc.GetName(), "namespace", pvc.GetNamespace(), "GVK", pvc.GetObjectKind().GroupVersionKind().String())

	compInfo, err := checkUnmanagedObject(ctx, pvc, p.client, p.controlPlaneClient, l)
	if err != nil {
		return nil, err
	}

	if compInfo.Exists {
		l.Info("Blocking deletion of PVC", "parent", compInfo.Name)
		return nil, fmt.Errorf(protectedMessage, "pvc", compInfo.Name)
	}

	l.Info("Allowing deletion of PVC", "parent", compInfo.Name)

	return nil, nil
}
