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
)

var (
	redisGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNRedis"}
	redisGR = schema.GroupResource{Group: pgGK.Group, Resource: "vshnredis"}
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
func (r *RedisWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) error {

	pg, ok := obj.(*vshnv1.VSHNRedis)
	if !ok {
		return fmt.Errorf("Provided manifest is not a valid VSHNRedis object")
	}

	if r.withQuota {
		err := r.checkQuotas(ctx, pg, true)
		if err != nil {
			return err
		}
	}

	return nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *RedisWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {

	redis, ok := newObj.(*vshnv1.VSHNRedis)
	if !ok {
		return fmt.Errorf("Provided manifest is not a valid VSHNRedis object")
	}

	if r.withQuota {
		err := r.checkQuotas(ctx, redis, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *RedisWebhookHandler) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	// NOOP for now
	return nil
}

// checkQuotas will read the plan if it's set and then check if any other size parameters are overwriten
func (r *RedisWebhookHandler) checkQuotas(ctx context.Context, redis *vshnv1.VSHNRedis, checkNamespaceQuota bool) *apierrors.StatusError {

	var fieldErr *field.Error
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
			pgGK,
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
		pgGR,
		pgGK,
		checkNamespaceQuota,
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
