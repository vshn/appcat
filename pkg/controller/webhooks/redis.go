package webhooks

import (
	"context"
	"fmt"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
			),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *RedisWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	comp, ok := obj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	if r.withQuota {
		quotaErrs := r.checkQuotas(ctx, comp, true)
		if quotaErrs != nil {
			allErrs = append(allErrs, &field.Error{
				Field: "quota",
				Detail: fmt.Sprintf("quota check failed: %s",
					quotaErrs.Error()),
				BadValue: "*your namespace quota*",
				Type:     field.ErrorTypeForbidden,
			})
		}
	}
	// k8s limitation is 52 characters, our longest postfix we add is 15 character, therefore 37 chracters is the maximum length
	err := r.validateResourceNameLength(comp.GetName(), 37)
	if err != nil {
		allErrs = append(allErrs, &field.Error{
			Field: ".metadata.name",
			Detail: fmt.Sprintf("Please shorten Redis name, currently it is: %s",
				err.Error()),
			BadValue: comp.GetName(),
			Type:     field.ErrorTypeTooLong,
		})
	}

	// We aggregate and return all errors at the same time.
	// So the user is aware of all broken parameters.
	// But at the same time, if any of these fail we cannot do proper quota checks anymore.
	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			r.gk,
			comp.GetName(),
			allErrs,
		)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *RedisWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	comp, ok := newObj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	if comp.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	if r.withQuota {
		quotaErrs := r.checkQuotas(ctx, comp, true)
		if quotaErrs != nil {
			allErrs = append(allErrs, &field.Error{
				Field: "quota",
				Detail: fmt.Sprintf("quota check failed: %s",
					quotaErrs.Error()),
				BadValue: "*your namespace quota*",
				Type:     field.ErrorTypeForbidden,
			})
		}
	}

	// k8s limitation is 52 characters, our longest postfix we add is 15 character, therefore 37 chracters is the maximum length
	err := r.validateResourceNameLength(comp.GetName(), 37)
	if err != nil {
		allErrs = append(allErrs, &field.Error{
			Field: ".metadata.name",
			Detail: fmt.Sprintf("Please shorten "+cases.Title(language.English).String(r.name)+" name, currently it is: %s",
				err.Error()),
			BadValue: comp.GetName(),
			Type:     field.ErrorTypeTooLong,
		})
	}

	// We aggregate and return all errors at the same time.
	// So the user is aware of all broken parameters.
	// But at the same time, if any of these fail we cannot do proper quota checks anymore.
	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			r.gk,
			comp.GetName(),
			allErrs,
		)
	}

	return nil, nil
}
