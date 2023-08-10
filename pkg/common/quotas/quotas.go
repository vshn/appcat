package quotas

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	orgLabelName = "appuio.io/organization"

	// Namespace related quotas
	defaultMaxNamespaces    = 25
	overrideCMDataFieldName = "namespaceQuota"
	nsOverrideCMPrefix      = "override-"
	nsOverrideCMNamespace   = "appuio-cloud"

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
	cpuRequestAnnotation    = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameCompute, quotaResourceCPURequests)
	cpuLimitAnnotation      = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameCompute, quotaResourceCPULimits)
	memoryRequestAnnotation = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameCompute, quotaResourceMemoryRequests)
	memoryLimitAnnotation   = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameCompute, quotaResourceMemoryLimits)
	diskAnnotation          = fmt.Sprintf("%s%s.%s", quotaAnnotationPrefix, resourceQuotaNameObjects, quotaResourceDisk)

	errNSLimitReached = fmt.Errorf("creating a new instance will violate the namespace quota." +
		"Please contact VSHN support to increase the amounts of namespaces you can create.")

	// These defaults allow up to a PostgreSQL or Redis standard-8 with one replica.

	// defaultCPURequests 2* standard-8 will request 4 CPUs. This default has 500m as spare for jobs
	defaultCPURequests = resource.NewMilliQuantity(4500, resource.DecimalSI)
	// defaultCPULimit by default same as DefaultCPURequests
	defaultCPULimit = defaultCPURequests
	// defaultMemoryRequests 2* standard-8 will request 16Gb. This default has 500mb as spare for jobs
	defaultMemoryRequests = resource.NewQuantity(17301504000, resource.BinarySI)
	// defaultMemoryLimits same as DefaultMemoryRequests
	defaultMemoryLimits = defaultMemoryRequests
	// defaultDiskRequests should be plenty for a large amount of replicas for any service
	defaultDiskRequests = resource.NewQuantity(1099511627776, resource.DecimalSI)
)

// QuotaChecker can check the given resources against the quotas set on an intance namespace
type QuotaChecker struct {
	requestedResources  Resources
	instanceNamespace   string
	claimNamespace      string
	claimName           string
	client              client.Client
	gr                  schema.GroupResource
	gk                  schema.GroupKind
	checkNamespaceQuota bool
}

// NewQuotaChecker creates a new quota checker.
// checkNamespaceQuota specifies whether or not the checker should also take the amount of existing
// namespaces into account, or not.
func NewQuotaChecker(c client.Client, claimName, claimNamespace, instanceNamespace string, r Resources, gr schema.GroupResource, gk schema.GroupKind, checkNamespaceQuota bool) QuotaChecker {
	return QuotaChecker{
		requestedResources:  r,
		instanceNamespace:   instanceNamespace,
		claimNamespace:      claimNamespace,
		claimName:           claimName,
		client:              c,
		gr:                  gr,
		gk:                  gk,
		checkNamespaceQuota: checkNamespaceQuota,
	}
}

// CheckQuotas runs the given quotas against a namespace.
// It also checks if the amount of namepsaces is within the quota as well.
func (q *QuotaChecker) CheckQuotas(ctx context.Context) *apierrors.StatusError {

	orgName, err := q.getOrgFromNamespace(ctx, q.claimNamespace)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	nsErr := q.areNamespacesWithinQuota(ctx, orgName)
	// If we've reached the namespace quotas, we're already done here.
	// Doesn't make sense to further check for quotas as we should not allow this
	// instance anyway.
	if nsErr != nil {
		return nsErr
	}

	return q.requestedResources.CheckResourcesAgainstQuotas(ctx, q.client, q.claimName, q.instanceNamespace, q.gk)
}

// areNamespacesWithinQuota checks for the given organization if there are still enough namespaces available.
func (q QuotaChecker) areNamespacesWithinQuota(ctx context.Context, orgName string) *apierrors.StatusError {

	l := ctrl.LoggerFrom(ctx)

	orgReq, err := labels.NewRequirement(orgLabelName, selection.Equals, []string{orgName})
	if err != nil {
		return apierrors.NewBadRequest(err.Error())
	}

	selector := labels.NewSelector().Add(*orgReq)

	l.V(1).Info("querying namespaces with selector", "labelSelector", selector.String())

	nsList := corev1.NamespaceList{}
	err = q.client.List(ctx, &nsList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return apierrors.NewBadRequest(err.Error())
	}

	foundNamespaces := len(nsList.Items)

	found, override, err := q.getNamespaceOverrides(ctx, q.client, orgName)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	if found {
		if foundNamespaces >= override {
			return apierrors.NewForbidden(q.gr, q.claimName, errNSLimitReached)
		}
		return nil
	}

	if foundNamespaces >= defaultMaxNamespaces {
		return apierrors.NewForbidden(q.gr, q.claimName, errNSLimitReached)
	}

	return nil
}

// getOrgFromNamespace gets the organization from the namespace.
// If there's an issue with getting the namespace or if the label is not set, it will return an error.
func (q *QuotaChecker) getOrgFromNamespace(ctx context.Context, nsName string) (string, error) {
	ns := &corev1.Namespace{}

	err := q.client.Get(ctx, client.ObjectKey{Name: nsName}, ns)
	if err != nil {
		return "", fmt.Errorf("cannot get namespace: %w", err)
	}

	nsOrg, ok := ns.GetLabels()[orgLabelName]
	if !ok {
		return "", fmt.Errorf("namespace does not have organization label")
	}
	return nsOrg, nil
}

// getNamespaceOverrides tries to get overrides for the given organization.
// It returns true and if it found an override alongside the value of the the override.
func (q *QuotaChecker) getNamespaceOverrides(ctx context.Context, c client.Client, orgName string) (bool, int, error) {
	cm := &corev1.ConfigMap{}

	err := c.Get(ctx, client.ObjectKey{Name: nsOverrideCMPrefix + orgName, Namespace: nsOverrideCMNamespace}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	override, ok := cm.Data[overrideCMDataFieldName]
	if !ok {
		return false, 0, nil
	}

	overrideInt, err := strconv.Atoi(override)
	if err != nil {
		return false, 0, err
	}

	return true, overrideInt, nil
}

// AddInitalNamespaceQuotas will add the default quotas to the namespace annotations.
// It will only add them, if there are currently no such annotations in place.
func AddInitalNamespaceQuotas(ns *corev1.Namespace) {
	annotations := ns.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	if _, ok := annotations[diskAnnotation]; !ok {
		annotations[diskAnnotation] = defaultDiskRequests.String()
	}

	if _, ok := annotations[cpuRequestAnnotation]; !ok {
		annotations[cpuRequestAnnotation] = defaultCPURequests.String()
	}

	if _, ok := annotations[cpuLimitAnnotation]; !ok {
		annotations[cpuLimitAnnotation] = defaultCPULimit.String()
	}

	if _, ok := annotations[memoryRequestAnnotation]; !ok {
		annotations[memoryRequestAnnotation] = defaultMemoryRequests.String()
	}

	if _, ok := annotations[memoryLimitAnnotation]; !ok {
		annotations[memoryLimitAnnotation] = defaultMemoryLimits.String()
	}

	ns.SetAnnotations(annotations)
}
