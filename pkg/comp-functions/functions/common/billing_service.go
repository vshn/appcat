package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	// BillingNamespace is the namespace where BillingService CRs are created
	BillingNamespace = "syn-appcat"
	// DefaultKeepAfterDeletion is the default number of days to keep billing records after deletion
	// it is overwritten by the component value appcat.billing.customResourceDeletionAfter
	DefaultKeepAfterDeletion = 365
)

// BillingServiceOptions contains customization options for creating a BillingService CR
type BillingServiceOptions struct {
	// ResourceNameSuffix is appended to comp.GetName() to form the resource name (e.g., "-billing-service", "-addon-collabora")
	ResourceNameSuffix string
	// Items defines the list of billable items/products for this instance
	Items []vshnv1.ItemSpec
	// AdditionalLabels are added to the BillingService CR labels
	AdditionalLabels map[string]string
}

// CreateOrUpdateBillingService creates or updates a BillingService CR for the given service instance.
// The salesOrder is populated from the composition for APPUiO Managed
// The Organisation is populated from claim namespace for APPUiO Cloud
// The productID is constructed as: appcat-vshn-{service}-{sla}
func CreateOrUpdateBillingService(ctx context.Context, svc *runtime.ServiceRuntime, comp InfoGetter) *xfnproto.Result {
	return CreateOrUpdateBillingServiceWithOptions(ctx, svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
}

// CreateOrUpdateBillingServiceWithOptions creates or updates a BillingService CR with custom options used for AddOns
func CreateOrUpdateBillingServiceWithOptions(ctx context.Context, svc *runtime.ServiceRuntime, comp InfoGetter, opts BillingServiceOptions) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Creating or updating BillingService", "service", comp.GetName())

	// Skip billing if disabled
	if svc.Config.Data["billingEnabled"] == "false" {
		return runtime.NewNormalResult(fmt.Sprintf("billing not enabled, skipping... %s", comp.GetName()))
	}

	// Skip billing for test instances
	if comp.GetClaimNamespace() == svc.Config.Data["ignoreNamespaceForBilling"] {
		log.Info("Test instance, skipping billing")
		return runtime.NewNormalResult(fmt.Sprintf("billing skipped for test instance %s", comp.GetName()))
	}

	claim := comp.GetClaimName()
	namespace := comp.GetClaimNamespace()
	service := comp.GetServiceName()

	// Get clusterName from config
	clusterName := svc.Config.Data["clusterName"]
	if clusterName == "" {
		log.Error(fmt.Errorf("missing billing clusterName"), "clusterName missing in composition")
		return runtime.NewWarningResult(fmt.Sprintf("no clusterName set in composition for %s", comp.GetName()))
	}

	// Get keepAfterDeletion from config
	keepAfterDeletion := DefaultKeepAfterDeletion
	if keepAfterDeletionStr := svc.Config.Data["crDeletionAfter"]; keepAfterDeletionStr != "" {
		if val, err := strconv.Atoi(keepAfterDeletionStr); err == nil {
			keepAfterDeletion = val
		}
	}

	isAPPUiOCloud := false
	salesOrder := svc.Config.Data["salesOrder"]
	if salesOrder == "" {
		isAPPUiOCloud = true
	}

	// salesOrderID annotation overrides config-level salesOrder before any description computation
	prefix := svc.Config.Data["servalaBillingAnnotationPrefix"]
	if prefix != "" {
		if v := comp.GetAnnotations()[prefix+"/salesOrderID"]; v != "" {
			salesOrder = v
			isAPPUiOCloud = false // explicit sales order — not APPUiO Cloud billing
		}
	}

	// Prepare ItemGroupDescription and ItemDescription for all items
	itemGroupDescription := claim
	itemDescription := GetItemDescription(isAPPUiOCloud, clusterName, namespace)

	// Clone to avoid mutating the caller's slice elements when filling in defaults
	items := make([]vshnv1.ItemSpec, len(opts.Items))
	copy(items, opts.Items)
	for i := range items {
		if items[i].ItemDescription == "" {
			items[i].ItemDescription = itemDescription
		}
		if items[i].ItemGroupDescription == "" {
			items[i].ItemGroupDescription = itemGroupDescription
		}
		if items[i].InstanceID == "" {
			items[i].InstanceID = comp.GetName() + "-" + shortSHA(items[i].ProductID)
		}
	}

	// Pre-fetch the existing billing service kube object before annotation checks.
	// This allows us to preserve it in the desired state even when annotation validation fails,
	// preventing Crossplane from garbage-collecting it.
	observedResourceName := comp.GetName() + opts.ResourceNameSuffix
	kubeObj := &xkube.Object{}
	observedErr := svc.GetObservedComposedResource(kubeObj, observedResourceName)
	if observedErr != nil && observedErr != runtime.ErrNotFound {
		log.Error(observedErr, "cannot get billing service kube object, treating as not found", "service", comp.GetName())
		observedErr = runtime.ErrNotFound // treat as not found; continue to create desired object
	}

	// preserveExistingAndWarn re-adds the existing billing service to the desired state (if it
	// exists) before returning a warning. This prevents Crossplane from garbage-collecting the
	// billing service when annotation validation fails.
	preserveExistingAndWarn := func(msg string) *xfnproto.Result {
		if observedErr == nil {
			if err := svc.SetDesiredComposedResourceWithName(kubeObj, observedResourceName); err != nil {
				log.Error(err, "cannot preserve existing billing service", "service", comp.GetName())
			}
		}
		return runtime.NewWarningResult(msg)
	}

	if prefix != "" {
		raw := comp.GetAnnotations()[prefix+"/items"]
		if raw == "" {
			return preserveExistingAndWarn(fmt.Sprintf("%s/items annotation missing on composite %s", prefix, comp.GetName()))
		}
		var payload struct {
			Items []vshnv1.ItemSpec `json:"items"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return preserveExistingAndWarn(fmt.Sprintf("cannot parse %s/items on %s: %v", prefix, comp.GetName(), err))
		}
		if len(payload.Items) == 0 {
			return preserveExistingAndWarn(fmt.Sprintf("%s/items is empty on composite %s", prefix, comp.GetName()))
		}
		for _, ai := range payload.Items {
			if ai.ProductID == "" {
				return preserveExistingAndWarn(fmt.Sprintf("%s/items contains an item with empty productID on composite %s", prefix, comp.GetName()))
			}
		}
		start := len(items)
		items = append(items, payload.Items...)
		for i := start; i < len(items); i++ {
			items[i].InstanceID = comp.GetName() + "-" + shortSHA(items[i].ProductID)
		}
	} else {
		// Default: compute a single standard product item (non-Servala clusters)
		productID := getProductID(comp, service)
		items = append(items, vshnv1.ItemSpec{
			ProductID:            productID,
			Value:                strconv.Itoa(comp.GetInstances()),
			ItemDescription:      itemDescription,
			ItemGroupDescription: itemGroupDescription,
			InstanceID:           comp.GetName() + "-" + shortSHA(productID),
		})
	}

	// Build labels
	labels := map[string]string{
		"appcat.vshn.io/claim-name":      claim,
		"appcat.vshn.io/claim-namespace": namespace,
		"appcat.vshn.io/service-name":    service,
	}
	for k, v := range opts.AdditionalLabels {
		labels[k] = v
	}

	// Create BillingService CR
	billingService := &vshnv1.BillingService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + opts.ResourceNameSuffix,
			Namespace: BillingNamespace,
			Labels:    labels,
		},
		Spec: vshnv1.BillingServiceSpec{
			KeepAfterDeletion: keepAfterDeletion,
			Odoo: vshnv1.OdooSpec{
				ServiceID:    comp.GetName(),
				SalesOrderID: salesOrder,
				Items:        items,
			},
		},
	}

	// Get organization (APPUiO cloud)
	if isAPPUiOCloud {
		org, err := GetOrg(comp.GetName(), svc)
		if err != nil {
			log.Error(err, "billing sales order and organization are missing", "service", comp.GetName())
			return runtime.NewWarningResult(fmt.Sprintf("cannot add billing to service %s", comp.GetName()))
		}
		billingService.Spec.Odoo.Organization = org
	}

	kubeOpts := []runtime.KubeObjectOption{
		runtime.KubeOptionDeployOnControlPlane,
		runtime.KubeOptionObserveCreateUpdate,
	}
	if observedErr == nil {
		// Create owner reference pointing to the Crossplane Object itself
		ownerRef := metav1.OwnerReference{
			APIVersion:         "kubernetes.crossplane.io/v1alpha2",
			Kind:               "Object",
			Name:               kubeObj.GetName(),
			UID:                kubeObj.GetUID(),
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(false),
		}
		kubeOpts = append(kubeOpts, runtime.KubeOptionSetOwnerReferenceFromKubeObject(billingService, ownerRef))
	}

	// Set the BillingService as a desired kube object
	if err := svc.SetDesiredKubeObject(billingService, observedResourceName, kubeOpts...); err != nil {
		log.Error(err, "cannot set BillingService as desired object", "service", comp.GetName())
		return preserveExistingAndWarn(fmt.Sprintf("cannot create BillingService for %s: %v", comp.GetName(), err))
	}

	return runtime.NewNormalResult(fmt.Sprintf("BillingService configured for instance %s with %d items", comp.GetName(), len(items)))
}

// GetItemDescription returns item description with cluster and namespace name
func GetItemDescription(isAPPUiOCloud bool, cluster, namespace string) string {
	if isAPPUiOCloud {
		return fmt.Sprintf("APPUiO Cloud - Cluster: %s / Namespace: %s", cluster, namespace)
	}
	return fmt.Sprintf("APPUiO Managed - Cluster: %s / Namespace: %s", cluster, namespace)
}

func getProductID(comp InfoGetter, service string) string {
	sla := vshnv1.BestEffort
	if comp.GetInstances() > 1 && comp.GetSLA() == string(vshnv1.Guaranteed) {
		sla = vshnv1.Guaranteed
	}

	// Construct productID: appcat-vshn-{service}-{sla}
	productID := fmt.Sprintf("appcat-vshn-%s-%s", service, sla)
	return productID
}

// shortSHA returns the first 8 hex characters of the SHA-256 hash of s.
func shortSHA(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:8]
}
