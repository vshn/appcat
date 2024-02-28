package utils

import (
	"context"
	"fmt"
	"strings"

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

const (
	OrgLabelName = "appuio.io/organization"

	// Namespace related quotas
	DefaultMaxNamespaces    = 25
	OverrideCMDataFieldName = "namespaceQuota"
	NsOverrideCMPrefix      = "override-"
	NsOverrideCMNamespace   = "appuio-cloud"

	// Resource related quota annotations
	// general form: resourcequota.appuio.io/<resourceQuotaName>.<resource>

	// resourceQuotaNameCompute resourceQuotaName for compute related quotas
	resourceQuotaNameCompute = "organization-compute"
	// resourceQuotaNameObjects resourceQuotaName for object related quotas
	resourceQuotaNameObjects = "organization-objects"
	// quotaAnnotationPrefix is the prefix for the quota annotations
	quotaAnnotationPrefix = "resourcequota.appuio.io/"
	// quotaResourceDisk resource for the disk quota
	quotaResourceDisk = "requests.storage"
	// quotaResourceCPURequests resource for the cpu requests quota
	quotaResourceCPURequests = "requests.cpu"
	// quotaResourceCPULimits resource for the cpu limit quota
	quotaResourceCPULimits = "limits.cpu"
	// quotaResourceMemoryRequests resource for the memory requests quota
	quotaResourceMemoryRequests = "requests.memory"
	// quotaResourceMemoryLimits resource for the memory limit quota
	quotaResourceMemoryLimits = "limits.memory"

	// Message snippets
	contactSupportMessage = "Please reduce the resources and then contact VSHN support to increase the quota for the instance support@vshn.ch."
)

var (
	// Now all the permutations for the annotations
	CpuRequestAnnotation       = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameCompute, quotaResourceCPURequests)
	CpuLimitAnnotation         = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameCompute, quotaResourceCPULimits)
	MemoryRequestAnnotation    = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameCompute, quotaResourceMemoryRequests)
	MemoryLimitAnnotation      = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameCompute, quotaResourceMemoryLimits)
	DiskAnnotation             = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameObjects, quotaResourceDisk)
	CpuRequestTerminationQuota = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameCompute+"-terminating", quotaResourceCPURequests)

	ErrNSLimitReached = fmt.Errorf("creating a new instance will violate the namespace quota." +
		"Please contact VSHN support to increase the amounts of namespaces you can create.")

	// These defaults allow up to a PostgreSQL or Redis standard-8 with one replica.

	// defaultCPURequests 2* standard-8 will request 4 CPUs. This default has 500m as spare for jobs
	DefaultCPURequests = resource.NewMilliQuantity(4500, resource.DecimalSI)
	// defaultCPULimit by default same as DefaultCPURequests
	DefaultCPULimits = DefaultCPURequests
	// defaultMemoryRequests 2* standard-8 will request 16Gb. This default has 500mb as spare for jobs
	DefaultMemoryRequests = resource.NewQuantity(17301504000, resource.BinarySI)
	// defaultMemoryLimits same as DefaultMemoryRequests
	DefaultMemoryLimits = DefaultMemoryRequests
	// defaultDiskRequests should be plenty for a large amount of replicas for any service
	DefaultDiskRequests = resource.NewQuantity(1099511627776, resource.DecimalSI)
)

// CheckResourcesAgainstQuotas will check the given resources either against:
// The resources in the instanceNamespace, if it's found
// Or against the default quotas, if not found
// The second case is usually triggered if a new instance is created, as we don't have a
// namespace to check against.
// Once the namespace exists, the composition should ensure that the annotations are set.
func (r *Resources) CheckResourcesAgainstQuotas(ctx context.Context, c client.Client, claimName, instanceNamespace string, gk schema.GroupKind, instances int64) *apierrors.StatusError {

	var nsQuotas Resources
	s := &Sidecars{}
	if instanceNamespace != "" {
		q, err := r.getNamespaceQuotas(ctx, c, instanceNamespace)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return apierrors.NewInternalError(err)
			}
		}
		nsQuotas = q
	} else {
		if gk.Kind == "VSHNPostgreSQL" {
			var err error
			s, err = FetchSidecarsFromCluster(ctx, c, "vshnpostgresqlplans")
			if err != nil {
				return apierrors.NewInternalError(err)
			}
		}
		nsQuotas = *GetDefaultResources(gk.Kind, s)
	}

	quotaErrs := r.checkAgainstResources(nsQuotas, gk.Kind, s)

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

	cpuReqString, ok := annotations[CpuRequestAnnotation]
	if ok {
		cpuRequests, err := resource.ParseQuantity(cpuReqString)
		if err != nil {
			return Resources{}, err
		}
		foundRes.CPURequests = cpuRequests
	}

	cpuLimitString, ok := annotations[CpuLimitAnnotation]
	if ok {
		cpuLimits, err := resource.ParseQuantity(cpuLimitString)
		if err != nil {
			return Resources{}, err
		}
		foundRes.CPULimits = cpuLimits
	}

	memoryReqString, ok := annotations[MemoryRequestAnnotation]
	if ok {
		memoryRequests, err := resource.ParseQuantity(memoryReqString)
		if err != nil {
			return Resources{}, err
		}
		foundRes.MemoryRequests = memoryRequests
	}

	memoryLimitString, ok := annotations[MemoryLimitAnnotation]
	if ok {
		memoryLimits, err := resource.ParseQuantity(memoryLimitString)
		if err != nil {
			return Resources{}, err
		}
		foundRes.MemoryLimits = memoryLimits
	}

	diskString, ok := annotations[DiskAnnotation]
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
func (r *Resources) checkAgainstResources(quotaResources Resources, kind string, s *Sidecars) field.ErrorList {
	foundErrs := field.ErrorList{}

	rDef := GetDefaultResources(kind, s)

	errCPURequests := field.Forbidden(r.CPURequestsPath, "Max allowed CPU requests: "+quotaResources.CPURequests.String()+". Configured requests: "+r.CPURequests.String()+". "+contactSupportMessage)

	if !quotaResources.CPURequests.IsZero() {
		if r.CPURequests.Cmp(quotaResources.CPURequests) == 1 {
			foundErrs = append(foundErrs, errCPURequests)
		}
	} else if r.CPURequests.Cmp(rDef.CPURequests) == 1 {
		foundErrs = append(foundErrs, errCPURequests)
	}

	errCPULimits := field.Forbidden(r.CPULimitsPath, "Max allowed CPU limits: "+quotaResources.CPULimits.String()+". Configured limits: "+r.CPULimits.String()+". "+contactSupportMessage)
	if !quotaResources.CPULimits.IsZero() {
		if r.CPULimits.Cmp(quotaResources.CPULimits) == 1 {
			foundErrs = append(foundErrs, errCPULimits)
		}
	} else if r.CPULimits.Cmp(rDef.CPULimits) == 1 {
		foundErrs = append(foundErrs, errCPULimits)
	}

	errMemoryRequests := field.Forbidden(r.MemoryRequestsPath, "Max allowed Memory requests: "+quotaResources.MemoryRequests.String()+". Configured requests: "+r.MemoryRequests.String()+". "+contactSupportMessage)
	if !quotaResources.MemoryRequests.IsZero() {
		if r.MemoryRequests.Cmp(quotaResources.MemoryRequests) == 1 {
			foundErrs = append(foundErrs, errMemoryRequests)
		}
	} else if r.MemoryRequests.Cmp(rDef.MemoryRequests) == 1 {
		foundErrs = append(foundErrs, errMemoryRequests)
	}

	errMemoryLimits := field.Forbidden(r.MemoryLimitsPath, "Max allowed Memory limits: "+quotaResources.MemoryLimits.String()+". Configured limits: "+r.MemoryLimits.String()+". "+contactSupportMessage)
	if !quotaResources.MemoryLimits.IsZero() {
		if r.MemoryLimits.Cmp(quotaResources.MemoryLimits) == 1 {
			foundErrs = append(foundErrs, errMemoryLimits)
		}
	} else if r.MemoryLimits.Cmp(rDef.MemoryLimits) == 1 {
		foundErrs = append(foundErrs, errMemoryLimits)
	}

	errDisk := field.Forbidden(r.DiskPath, "Max allowed Disk: "+quotaResources.Disk.String()+". Configured requests: "+r.Disk.String()+". "+contactSupportMessage)
	if !quotaResources.Disk.IsZero() {
		if r.Disk.Cmp(quotaResources.Disk) == 1 {
			foundErrs = append(foundErrs, errDisk)
		}
	} else if r.Disk.Cmp(*DefaultDiskRequests) == 1 {
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

func (r *Resources) AddResources(resource Resources) {
	r.CPULimits.Add(resource.CPULimits)
	r.CPURequests.Add(resource.CPURequests)
	r.MemoryLimits.Add(resource.MemoryLimits)
	r.MemoryRequests.Add(resource.MemoryRequests)
	r.Disk.Add(resource.Disk)
}

// AddPsqlSidecarResources adds the resource overhead for the PostgreSQL sidecar to the given resource.
func (r *Resources) AddPsqlSidecarResources(s *Sidecars, instances int64) {
	resourcesSidecars, err := GetAllSideCarsResources(s)
	if err != nil {
		instances = 1
	}
	r.CPURequests.Add(*resource.NewMilliQuantity(resourcesSidecars.CPURequests.MilliValue()*instances, resource.DecimalSI))
	r.MemoryRequests.Add(*resource.NewQuantity(resourcesSidecars.MemoryRequests.Value()*instances, resource.BinarySI))

	// Sidecar limits: 4800m CPU, 6144Mi Memory
	r.CPULimits.Add(*resource.NewMilliQuantity(resourcesSidecars.CPULimits.MilliValue()*instances, resource.DecimalSI))
	r.MemoryLimits.Add(*resource.NewQuantity(resourcesSidecars.MemoryLimits.Value()*instances, resource.BinarySI))
}

// GetDefaultResources returns a new Resources struct with the default values.
func GetDefaultResources(kind string, s *Sidecars) *Resources {
	r := &Resources{
		CPURequests:    *DefaultCPURequests,
		CPULimits:      *DefaultCPULimits,
		MemoryRequests: *DefaultMemoryRequests,
		MemoryLimits:   *DefaultMemoryLimits,
	}
	if strings.Contains(kind, "VSHNPostgreSQL") {
		r.AddPsqlSidecarResources(s, int64(2))
	}
	return r
}
