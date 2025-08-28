package webhooks

import (
	"context"
	"fmt"
	"strings"

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

	// Check if this is a backup-related resource that should be allowed to be deleted
	// when backup is disabled (except for XObjectBucket which has its own webhook)
	if isBackupRelatedResource(resource) {
		l.Info("Allowing deletion of backup-related resource", "resource", resource.GetName())
		return nil, nil
	}

	compInfo, err := checkManagedObject(ctx, resource, p.client, p.controlPlaneClient, l)
	if err != nil {
		return nil, err
	}

	if compInfo.Exists {
		message := protectedMessage
		if compInfo.Reason != "" {
			message = compInfo.Reason
		}
		l.Info("Blocking deletion of resource "+resource.GetName(), "parent", compInfo.Name, "reason", message)
		return nil, fmt.Errorf(message, "release", compInfo.Name)
	}

	l.Info("Allowing deletion of resource "+resource.GetName(), "parent", compInfo.Name)

	return nil, nil
}

// isBackupRelatedResource checks if a resource is backup-related and should be allowed
// to be deleted when backup is disabled (except for XObjectBucket which has its own webhook)
func isBackupRelatedResource(resource client.Object) bool {
	resourceName := resource.GetName()

	// Allow deletion of k8up repository password secrets
	if strings.HasSuffix(resourceName, "-k8up-repo-pw") {
		return true
	}

	// Allow deletion of backup schedules
	if strings.Contains(resourceName, "-backup-schedule") {
		return true
	}

	// Allow deletion of backup scripts
	if strings.HasSuffix(resourceName, "-backup-script") {
		return true
	}

	// Allow deletion of backup credential secrets
	if strings.Contains(resourceName, "backup-bucket-credentials") {
		return true
	}

	return false
}
