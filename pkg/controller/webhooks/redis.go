package webhooks

import (
	"context"
	"fmt"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=create;update;	delete,path=/validate-vshn-appcat-vshn-io-v1-vshnredis,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnredis,versions=v1,name=vshnredis.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnredis,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnredis/status,verbs=get;list;watch;patch;update

var (
	redisGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNRedis"}
	redisGR = schema.GroupResource{Group: redisGK.Group, Resource: "vshnredis"}
)

var _ webhook.CustomValidator = &RedisWebhookHandler{}

// RedisWebhookHandler handles all quota webhooks concerning redis by vshn.
type RedisWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupRedisWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupRedisWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNRedis{}).
		WithValidator(&RedisWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("redis"),
				withQuota,
				&vshnv1.VSHNRedis{},
				"redis",
				redisGK,
				redisGR,
				maxResourceNameLength,
			),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *RedisWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	comp, ok := obj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	allErrs := newFielErrors(comp.GetName(), redisGK)

	// First call the parent validation (includes provider config validation)
	parentWarnings, parentErr := r.DefaultWebhookHandler.ValidateCreate(ctx, obj)
	if parentErr != nil {
		tmpErr := parentErr.(*fieldErrors)
		allErrs.Add(tmpErr.List()...)
	}
	if parentWarnings != nil && parentErr == nil {
		return parentWarnings, nil
	}

	return nil, allErrs.Get()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *RedisWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	comp, ok := newObj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	if comp.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	allErrs := newFielErrors(comp.GetName(), redisGK)

	// First call the parent validation (includes provider config validation)
	parentWarnings, parentErr := r.DefaultWebhookHandler.ValidateUpdate(ctx, oldObj, newObj)
	if parentErr != nil {
		tmpErr := parentErr.(*fieldErrors)
		allErrs.Add(tmpErr.List()...)
	}
	if parentWarnings != nil && parentErr == nil {
		return parentWarnings, nil
	}

	return nil, allErrs.Get()
}
