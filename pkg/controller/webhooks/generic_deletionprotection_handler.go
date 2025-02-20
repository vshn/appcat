package webhooks

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ webhook.CustomValidator = &GenericDeletionProtectionHandler{}

type GenericDeletionProtectionHandler struct {
	client             client.Client
	controlPlaneClient client.Client
	log                logr.Logger
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *GenericDeletionProtectionHandler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *GenericDeletionProtectionHandler) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (p *GenericDeletionProtectionHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {

	resource, ok := obj.(client.Object)
	if !ok {
		return nil, fmt.Errorf("object is not valid")
	}

	l := p.log.WithValues("object", resource.GetName(), "object", resource.GetNamespace(), "GVK", resource.GetObjectKind().GroupVersionKind().String())

	compInfo, err := checkManagedObject(ctx, resource, p.client, p.controlPlaneClient, l)
	if err != nil {
		return nil, err
	}

	if compInfo.Exists {
		l.Info("Blocking deletion of resource "+resource.GetName(), "parent", compInfo.Name)
		return nil, fmt.Errorf(protectedMessage, "release", compInfo.Name)
	}

	l.Info("Blocking deletion of resource "+resource.GetName(), "parent", compInfo.Name)
	return nil, fmt.Errorf(protectedMessage, "release", compInfo.Name)

	// l.Info("Allowing deletion of resource "+resource.GetName(), "parent", compInfo.Name)

	// return nil, nil
}
