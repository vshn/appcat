package webhooks

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=delete;update,path=/validate-appcat-vshn-io-v1-objectbucket,mutating=false,failurePolicy=fail,groups=appcat.vshn.io,resources=objectbuckets,versions=v1,name=objectbuckets.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1
//+kubebuilder:rbac:groups=cloudscale.crossplane.io,resources=providerconfigs,verbs=get;list;watch;
//+kubebuilder:rbac:groups=exoscale.crossplane.io,resources=providerconfigs,verbs=get;list;watch;

var _ webhook.CustomValidator = &ObjectbucketDeletionProtectionHandler{}

// ObjectbucketDeletionProtectionHandler
type ObjectbucketDeletionProtectionHandler struct {
	client client.Client
	log    logr.Logger
}

// SetupObjectbucketDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupObjectbucketDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&appcatv1.ObjectBucket{}).
		WithValidator(&ObjectbucketDeletionProtectionHandler{
			client: mgr.GetClient(),
			log:    mgr.GetLogger().WithName("webhook").WithName("objectbucket"),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *ObjectbucketDeletionProtectionHandler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *ObjectbucketDeletionProtectionHandler) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldBucket, ok := oldObj.(*v1.ObjectBucket)
	if !ok {
		return nil, fmt.Errorf("old object is not valid")
	}

	newBucket, ok := newObj.(*v1.ObjectBucket)
	if !ok {
		return nil, fmt.Errorf("new object is not valid")
	}

	allErrs := newFielErrors(newBucket.GetName(), oldBucket.GroupVersionKind().GroupKind())

	// Prevent changing bucketName after creation
	if oldBucket.Spec.Parameters.BucketName != newBucket.Spec.Parameters.BucketName {
		allErrs.Add(field.Invalid(
			field.NewPath("spec", "parameters", "bucketName"),
			newBucket.Spec.Parameters.BucketName,
			"bucketName cannot be changed after bucket is created",
		))
	}

	return nil, allErrs.Get()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (p *ObjectbucketDeletionProtectionHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	bucket, ok := obj.(*v1.ObjectBucket)
	if !ok {
		return nil, fmt.Errorf("object is not valid")
	}

	allErr := newFielErrors(bucket.GetName(), bucket.GroupVersionKind().GroupKind())

	if err := GetClaimDeletionProtection(&bucket.Spec.Parameters.Security); err != nil {
		allErr.Add(err)
	}

	return nil, allErr.Get()
}
