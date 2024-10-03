package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=delete,path=/validate-appcat-vshn-io-v1-xobjectbucket,mutating=false,failurePolicy=fail,groups=appcat.vshn.io,resources=xobjectbuckets,versions=v1,name=xobjectbuckets.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.CustomValidator = &XObjectbucketDeletionProtectionHandler{}

// XObjectbucketDeletionProtectionHandler
type XObjectbucketDeletionProtectionHandler struct {
	client client.Client
	log    logr.Logger
}

// SetupXObjectbucketCDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupXObjectbucketCDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&appcatv1.XObjectBucket{}).
		WithValidator(&XObjectbucketDeletionProtectionHandler{
			client: mgr.GetClient(),
			log:    mgr.GetLogger().WithName("webhook").WithName("objectbucket"),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *XObjectbucketDeletionProtectionHandler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *XObjectbucketDeletionProtectionHandler) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (p *XObjectbucketDeletionProtectionHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {

	bucket, ok := obj.(client.Object)
	if !ok {
		return nil, fmt.Errorf("object is not valid")
	}

	creationTimestamp := bucket.GetCreationTimestamp()
	allowedDeletionTime := creationTimestamp.Add(61 * time.Minute)

	now := time.Now()
	age := now.Sub(creationTimestamp.Time)

	if age < 61*time.Minute {
		return nil, fmt.Errorf("XObjectBucket is too young to be deleted, need to wait another %.1f minutes to ensure correct billing", allowedDeletionTime.Sub(now).Minutes())
	}

	l := p.log.WithValues("object", bucket.GetName(), "namespace", bucket.GetNamespace(), "GVK", bucket.GetObjectKind().GroupVersionKind().String())

	compInfo, err := checkManagedObject(ctx, bucket, p.client, l)
	if err != nil {
		return nil, err
	}

	if compInfo.Exists {
		l.Info("Blocking deletion of XObjectBucket", "parent", compInfo.Name)
		return nil, fmt.Errorf(protectedMessage, "XObjectBucket", compInfo.Name)
	}

	l.Info("Allowing deletion of XObjectBucket", "parent", compInfo.Name, "age", age.String())

	return nil, nil
}
