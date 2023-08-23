package quotas

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resources contains the Resources that the given instance will use.
// If the service has more than 1 replica then the values need to be adjusted.
type Resources struct {
	CPURequests        resource.Quantity
	CPURequestsPath    *field.Path
	CPULimits          resource.Quantity
	CPULimitsPath      *field.Path
	MemoryRequests     resource.Quantity
	MemoryRequestsPath *field.Path
	MemoryLimits       resource.Quantity
	MemoryLimitsPath   *field.Path
	Disk               resource.Quantity
	DiskPath           *field.Path
}

// CheckResourcesAgainstQuotas will check the given resources either against:
// The resources in the instanceNamespace, if it's found
// Or against the default quotas, if not found
// The second case is usually triggered if a new instance is created, as we don't have a
// namespace to check against.
// Once the namespace exists, the composition should ensure that the annotations are set.
func (r *Resources) CheckResourcesAgainstQuotas(ctx context.Context, c client.Client, claimName, instanceNamespace string, gk schema.GroupKind) *apierrors.StatusError {

	nsQuotas := Resources{}
	if instanceNamespace != "" {
		q, err := r.getNamespaceQuotas(ctx, c, instanceNamespace)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return apierrors.NewInternalError(err)
			}
		}
		nsQuotas = q
	}

	quotaErrs := r.checkAgainstResources(nsQuotas)

	if len(quotaErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		gk,
		claimName,
		quotaErrs)
}

// getNamespaceQuotas returns the quotas of a given namespace. If the namespace does not exist, it will return an apierror.errNotFound.
// It will return other errors if the values cannot be parsed.
// If there's an override missing for a quota, it will simply not populate the value in the resulting resources struct.
// If an override is set, but it cannot be parsed, we also return an error.
func (r *Resources) getNamespaceQuotas(ctx context.Context, c client.Client, instanceNamespace string) (Resources, error) {
	ns := &corev1.Namespace{}
	foundRes := Resources{}

	err := c.Get(ctx, client.ObjectKey{Name: instanceNamespace}, ns)
	if err != nil {
		return foundRes, err
	}

	annotations := ns.GetAnnotations()

	cpuReqString, ok := annotations[cpuRequestAnnotation]
	if ok {
		cpuRequests, err := resource.ParseQuantity(cpuReqString)
		if err != nil {
			return Resources{}, err
		}
		foundRes.CPURequests = cpuRequests
	}

	cpuLimitString, ok := annotations[cpuLimitAnnotation]
	if ok {
		cpuLimits, err := resource.ParseQuantity(cpuLimitString)
		if err != nil {
			return Resources{}, err
		}
		foundRes.CPULimits = cpuLimits
	}

	memoryReqString, ok := annotations[memoryRequestAnnotation]
	if ok {
		memoryRequests, err := resource.ParseQuantity(memoryReqString)
		if err != nil {
			return Resources{}, err
		}
		foundRes.MemoryRequests = memoryRequests
	}

	memoryLimitString, ok := annotations[memoryLimitAnnotation]
	if ok {
		memoryLimits, err := resource.ParseQuantity(memoryLimitString)
		if err != nil {
			return Resources{}, err
		}
		foundRes.MemoryLimits = memoryLimits
	}

	diskString, ok := annotations[diskAnnotation]
	if ok {
		disk, err := resource.ParseQuantity(diskString)
		if err != nil {
			return Resources{}, err
		}
		foundRes.Disk = disk
	}

	return foundRes, nil
}

// checkAgainstResources compares this resources against the given resources.
// Any non-populated fields are checked against their defaults.
func (r *Resources) checkAgainstResources(quotaResources Resources) field.ErrorList {
	foundErrs := field.ErrorList{}

	errCPURequests := field.Forbidden(r.CPURequestsPath, "Max allowed CPU requests: "+defaultCPURequests.String()+". Configured requests: "+r.CPURequests.String()+". "+contactSupportMessage)
	if !quotaResources.CPURequests.IsZero() {
		if r.CPURequests.Cmp(quotaResources.CPURequests) == 1 {
			foundErrs = append(foundErrs, errCPURequests)
		}
	} else if r.CPURequests.Cmp(*defaultCPURequests) == 1 {
		foundErrs = append(foundErrs, errCPURequests)
	}

	errCPULimits := field.Forbidden(r.CPULimitsPath, "Max allowed CPU limits: "+defaultCPULimit.String()+". Configured limits: "+r.CPULimits.String()+". "+contactSupportMessage)
	if !quotaResources.CPULimits.IsZero() {
		if r.CPULimits.Cmp(quotaResources.CPULimits) == 1 {
			foundErrs = append(foundErrs, errCPULimits)
		}
	} else if r.CPULimits.Cmp(*defaultCPULimit) == 1 {
		foundErrs = append(foundErrs, errCPULimits)
	}

	errMemoryRequests := field.Forbidden(r.MemoryRequestsPath, "Max allowed Memory requests: "+defaultMemoryRequests.String()+". Configured requests: "+r.MemoryRequests.String()+". "+contactSupportMessage)
	if !quotaResources.MemoryRequests.IsZero() {
		if r.MemoryRequests.Cmp(quotaResources.MemoryRequests) == 1 {
			foundErrs = append(foundErrs, errMemoryRequests)
		}
	} else if r.MemoryRequests.Cmp(*defaultMemoryRequests) == 1 {
		foundErrs = append(foundErrs, errMemoryRequests)
	}

	errMemoryLimits := field.Forbidden(r.MemoryLimitsPath, "Max allowed Memory limits: "+defaultMemoryLimits.String()+". Configured limits: "+r.MemoryLimits.String()+". "+contactSupportMessage)
	if !quotaResources.MemoryLimits.IsZero() {
		if r.MemoryLimits.Cmp(quotaResources.MemoryLimits) == 1 {
			foundErrs = append(foundErrs, errMemoryLimits)
		}
	} else if r.MemoryLimits.Cmp(*defaultMemoryLimits) == 1 {
		foundErrs = append(foundErrs, errMemoryLimits)
	}

	errDisk := field.Forbidden(r.DiskPath, "Max allowed Disk: "+defaultCPULimit.String()+". Configured requests: "+r.Disk.String()+". "+contactSupportMessage)
	if !quotaResources.Disk.IsZero() {
		if r.Disk.Cmp(quotaResources.Disk) == 1 {
			foundErrs = append(foundErrs, errDisk)
		}
	} else if r.Disk.Cmp(*defaultDiskRequests) == 1 {
		foundErrs = append(foundErrs, errDisk)
	}

	if len(foundErrs) == 0 {
		return nil
	}

	return foundErrs
}

// MultiplyBy multiplies the resources by given integer.
// if given integer is less or equal to 0 it will not do any operation.
func (r *Resources) MultiplyBy(i int64) {
	if i <= 0 {
		return
	}
	r.CPULimits.SetMilli(r.CPULimits.MilliValue() * i)
	r.CPURequests.SetMilli(r.CPURequests.MilliValue() * i)
	r.MemoryLimits.Set(r.MemoryLimits.Value() * i)
	r.MemoryRequests.Set(r.MemoryRequests.Value() * i)
	r.Disk.Set(r.Disk.Value() * i)
}
