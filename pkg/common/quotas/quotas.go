package quotas

import (
	"context"
	"fmt"
	"strconv"

	"github.com/vshn/appcat/v4/pkg/common/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// QuotaChecker can check the given resources against the quotas set on an intance namespace
type QuotaChecker struct {
	requestedResources  utils.Resources
	instanceNamespace   string
	claimNamespace      string
	claimName           string
	client              client.Client
	gr                  schema.GroupResource
	gk                  schema.GroupKind
	checkNamespaceQuota bool
	instances           int64
}

// NewQuotaChecker creates a new quota checker.
// checkNamespaceQuota specifies whether or not the checker should also take the amount of existing
// namespaces into account, or not.
func NewQuotaChecker(c client.Client, claimName, claimNamespace, instanceNamespace string, r utils.Resources, gr schema.GroupResource, gk schema.GroupKind, checkNamespaceQuota bool, instances int64) QuotaChecker {
	return QuotaChecker{
		requestedResources:  r,
		instanceNamespace:   instanceNamespace,
		claimNamespace:      claimNamespace,
		claimName:           claimName,
		client:              c,
		gr:                  gr,
		gk:                  gk,
		checkNamespaceQuota: checkNamespaceQuota,
		instances:           instances,
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

	return q.requestedResources.CheckResourcesAgainstQuotas(ctx, q.client, q.claimName, q.instanceNamespace, q.gk, q.instances)
}

// areNamespacesWithinQuota checks for the given organization if there are still enough namespaces available.
func (q QuotaChecker) areNamespacesWithinQuota(ctx context.Context, orgName string) *apierrors.StatusError {

	l := ctrl.LoggerFrom(ctx)

	orgReq, err := labels.NewRequirement(utils.OrgLabelName, selection.Equals, []string{orgName})
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
			return apierrors.NewForbidden(q.gr, q.claimName, utils.ErrNSLimitReached)
		}
		return nil
	}

	if foundNamespaces >= utils.DefaultMaxNamespaces {
		return apierrors.NewForbidden(q.gr, q.claimName, utils.ErrNSLimitReached)
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

	nsOrg, ok := ns.GetLabels()[utils.OrgLabelName]
	if !ok {
		return "", fmt.Errorf("namespace does not have organization label")
	}
	return nsOrg, nil
}

// getNamespaceOverrides tries to get overrides for the given organization.
// It returns true and if it found an override alongside the value of the the override.
func (q *QuotaChecker) getNamespaceOverrides(ctx context.Context, c client.Client, orgName string) (bool, int, error) {
	cm := &corev1.ConfigMap{}

	err := c.Get(ctx, client.ObjectKey{Name: utils.NsOverrideCMPrefix + orgName, Namespace: utils.NsOverrideCMNamespace}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	override, ok := cm.Data[utils.OverrideCMDataFieldName]
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
func AddInitalNamespaceQuotas(ctx context.Context, ns *corev1.Namespace, s *utils.Sidecars, kind string) bool {
	annotations := ns.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	added := false

	r := utils.GetDefaultResources(kind, s)

	if _, ok := annotations[utils.DiskAnnotation]; !ok {
		annotations[utils.DiskAnnotation] = utils.DefaultDiskRequests.String()
		added = true
	}

	if _, ok := annotations[utils.CpuRequestAnnotation]; !ok {
		annotations[utils.CpuRequestAnnotation] = r.CPURequests.String()
		added = true
	}

	if _, ok := annotations[utils.CpuLimitAnnotation]; !ok {
		annotations[utils.CpuLimitAnnotation] = r.CPULimits.String()
		added = true
	}

	if _, ok := annotations[utils.MemoryRequestAnnotation]; !ok {
		annotations[utils.MemoryRequestAnnotation] = r.MemoryRequests.String()
		added = true
	}

	if _, ok := annotations[utils.MemoryLimitAnnotation]; !ok {
		annotations[utils.MemoryLimitAnnotation] = r.MemoryLimits.String()
		added = true
	}

	if _, ok := annotations[utils.CpuRequestTerminationQuota]; !ok {
		annotations[utils.CpuRequestTerminationQuota] = "1000m"
		added = true
	}

	ns.SetAnnotations(annotations)
	return added
}
