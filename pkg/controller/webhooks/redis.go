package webhooks

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	redisGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNRedis"}
	redisGR = schema.GroupResource{Group: redisGK.Group, Resource: "vshnredis"}
)

var _ webhook.CustomValidator = &RedisWebhookHandler{}

// RedisWebhookHandler handles all quota webhooks concerning redis by vshn.
type RedisWebhookHandler struct {
	client    client.Client
	log       logr.Logger
	withQuota bool
}

// SetupRedisWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupRedisWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNRedis{}).
		WithValidator(&RedisWebhookHandler{
			client:    mgr.GetClient(),
			log:       mgr.GetLogger().WithName("webhook").WithName("redis"),
			withQuota: withQuota,
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *RedisWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	redis, ok := obj.(*vshnv1.VSHNRedis)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNRedis object")
	}

	if r.withQuota {
		quotaErrs := r.checkQuotas(ctx, redis, true)
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
	err := r.validateResourceNameLength(redis.GetName())
	if err != nil {
		allErrs = append(allErrs, &field.Error{
			Field: ".metadata.name",
			Detail: fmt.Sprintf("Please shorten Redis name, currently it is: %s",
				err.Error()),
			BadValue: redis.GetName(),
			Type:     field.ErrorTypeTooLong,
		})
	}

	// We aggregate and return all errors at the same time.
	// So the user is aware of all broken parameters.
	// But at the same time, if any of these fail we cannot do proper quota checks anymore.
	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			redisGK,
			redis.GetName(),
			allErrs,
		)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *RedisWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	redis, ok := newObj.(*vshnv1.VSHNRedis)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNRedis object")
	}

	if redis.DeletionTimestamp != nil {
		return nil, nil
	}

	if r.withQuota {
		quotaErrs := r.checkQuotas(ctx, redis, true)
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

	err := r.validateResourceNameLength(redis.GetName())
	if err != nil {
		allErrs = append(allErrs, &field.Error{
			Field: ".metadata.name",
			Detail: fmt.Sprintf("Please shorten Redis name, currently it is: %s",
				err.Error()),
			BadValue: redis.GetName(),
			Type:     field.ErrorTypeTooLong,
		})
	}

	// We aggregate and return all errors at the same time.
	// So the user is aware of all broken parameters.
	// But at the same time, if any of these fail we cannot do proper quota checks anymore.
	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			redisGK,
			redis.GetName(),
			allErrs,
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *RedisWebhookHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// checkQuotas will read the plan if it's set and then check if any other size parameters are overwriten
func (r *RedisWebhookHandler) checkQuotas(ctx context.Context, redis *vshnv1.VSHNRedis, checkNamespaceQuota bool) *apierrors.StatusError {

	var fieldErr *field.Error
	// TODO: Fix once we support replicas
	instances := int64(1)
	allErrs := field.ErrorList{}
	resources := utils.Resources{}

	if redis.Spec.Parameters.Size.Plan != "" {
		var err error
		resources, err = utils.FetchPlansFromCluster(ctx, r.client, "vshnredisplans", redis.Spec.Parameters.Size.Plan)
		if err != nil {
			return apierrors.NewInternalError(err)
		}
	}

	r.addPathsToResources(&resources)

	if redis.Spec.Parameters.Size.CPULimits != "" {
		resources.CPULimits, fieldErr = parseResource(resources.CPULimitsPath, redis.Spec.Parameters.Size.CPULimits, "not a valid cpu size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	if redis.Spec.Parameters.Size.CPURequests != "" {
		resources.CPURequests, fieldErr = parseResource(resources.CPURequestsPath, redis.Spec.Parameters.Size.CPURequests, "not a valid cpu size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	if redis.Spec.Parameters.Size.MemoryLimits != "" {
		resources.MemoryLimits, fieldErr = parseResource(resources.MemoryLimitsPath, redis.Spec.Parameters.Size.MemoryLimits, "not a valid memory size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	if redis.Spec.Parameters.Size.MemoryRequests != "" {
		resources.MemoryRequests, fieldErr = parseResource(resources.MemoryRequestsPath, redis.Spec.Parameters.Size.MemoryRequests, "not a valid memory size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	if redis.Spec.Parameters.Size.Disk != "" {
		resources.Disk, fieldErr = parseResource(resources.DiskPath, redis.Spec.Parameters.Size.Disk, "not a valid cpu size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	// We aggregate and return all errors at the same time.
	// So the user is aware of all broken parameters.
	// But at the same time, if any of these fail we cannot do proper quota checks anymore.
	if len(allErrs) != 0 {
		return apierrors.NewInvalid(
			redisGK,
			redis.GetName(),
			allErrs,
		)
	}

	// No support for replicas yet in redis
	// resources.MultiplyBy(instances)

	checker := quotas.NewQuotaChecker(
		r.client,
		redis.GetName(),
		redis.GetNamespace(),
		redis.Status.InstanceNamespace,
		resources,
		redisGR,
		redisGK,
		checkNamespaceQuota,
		instances,
	)

	return checker.CheckQuotas(ctx)
}

func (r *RedisWebhookHandler) addPathsToResources(res *utils.Resources) {
	basePath := field.NewPath("spec", "parameters", "size")

	res.CPULimitsPath = basePath.Child("CPULimits")
	res.CPURequestsPath = basePath.Child("CPURequests")
	res.MemoryLimitsPath = basePath.Child("memoryLimits")
	res.MemoryRequestsPath = basePath.Child("memoryLimits")
	res.DiskPath = basePath.Child("disk")
}

// k8s limitation is 52 characters, our longest postfix we add is 15 character, therefore 37 chracters is the maximum length
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
func (r *RedisWebhookHandler) validateResourceNameLength(name string) error {
	if len(name) > 37 {
		return fmt.Errorf("%d/37 chars.\n\tWe add various postfixes and CronJob name length has it's own limitations: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names", len(name))
	}
	return nil
}
