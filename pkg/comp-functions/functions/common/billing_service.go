package common

import (
	"context"
	"fmt"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	// BillingNamespace is the namespace where BillingService CRs are created
	BillingNamespace = "syn-appcat"
	// DefaultKeepAfterDeletion is the default number of days to keep billing records after deletion
	DefaultKeepAfterDeletion = 365
)

// CreateOrUpdateBillingService creates or updates a BillingService CR for the given service instance.
// It handles:
// - Creating a new BillingService CR when an instance is created
// - Updating the BillingService when replica count changes (which changes the productID)
// - Skipping billing for test instances (based on ignoreNamespaceForBilling config)
// - The deletion is handled automatically by the billing controller when the service is deleted
//
// The SLA is determined by instance count:
// - instances == 1: besteffort
// - instances > 1: guaranteed
//
// The salesOrder is populated from the composition for APPUiO Managed
// The Organisation is populated from claim namespace for APPUiO Cloud
//
// The productID is constructed as: appcat-vshn-{service}-{sla}
func CreateOrUpdateBillingService(ctx context.Context, svc *runtime.ServiceRuntime, comp InfoGetter) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Creating or updating BillingService", "service", comp.GetName())

	// Skip billing for test instances
	if comp.GetClaimNamespace() == svc.Config.Data["ignoreNamespaceForBilling"] {
		log.Info("Test instance, skipping billing")
		return runtime.NewNormalResult(fmt.Sprintf("billing skipped for test instance %s", comp.GetName()))
	}

	claim := comp.GetClaimName()
	namespace := comp.GetClaimNamespace()
	service := comp.GetServiceName()

	// Form productID from service and number of replicas
	productID := getProductID(comp.GetInstances(), service)

	// Get unitID from config
	unitID := svc.Config.Data["billingUnitID"]
	if unitID == "" {
		log.Error(fmt.Errorf("missing billing unitID"), "UnitID missing in composition")
		return runtime.NewNormalResult(fmt.Sprintf("no billing unit id set in composition for %s", comp.GetName()))
	}

	// Get clusterName from config
	clusterName := svc.Config.Data["clusterName"]
	if clusterName == "" {
		log.Error(fmt.Errorf("missing billing clusterName"), "clusterName missing in composition")
		return runtime.NewNormalResult(fmt.Sprintf("no clusterName set in composition for %s", comp.GetName()))
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

	// Create BillingService CR
	billingService := &vshnv1.BillingService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: BillingNamespace,
			Labels: map[string]string{
				"appcat.vshn.io/claim-name":      claim,
				"appcat.vshn.io/claim-namespace": namespace,
				"appcat.vshn.io/service-name":    service,
			},
		},
		Spec: vshnv1.BillingServiceSpec{
			KeepAfterDeletion: keepAfterDeletion,
			Odoo: vshnv1.OdooSpec{
				InstanceID:           comp.GetName(),
				ProductID:            productID,
				UnitID:               unitID,
				SalesOrderID:         salesOrder,
				ItemGroupDescription: claim,
				ItemDescription:      getItemDescription(isAPPUiOCloud, clusterName, namespace),
			},
		},
	}

	// Get organization (APPUiO cloud)
	if isAPPUiOCloud {
		org, err := getOrg(comp.GetName(), svc)
		if err != nil {
			log.Error(err, "billing sales order and organization is missing", "service", comp.GetName())
			return runtime.NewWarningResult(fmt.Sprintf("cannot add billing to service %s", comp.GetName()))
		}
		billingService.Spec.Odoo.Organization = org
	}

	// Set the BillingService as a desired kube object
	err := svc.SetDesiredKubeObject(billingService, comp.GetName()+"-billing-service",
		runtime.KubeOptionDeployOnControlPlane)

	if err != nil {
		log.Error(err, "cannot set BillingService as desired object", "service", comp.GetName())
		return runtime.NewWarningResult(fmt.Sprintf("cannot create BillingService for %s: %v", comp.GetName(), err))
	}

	return runtime.NewNormalResult(fmt.Sprintf("BillingService configured for instance %s", comp.GetName()))
}

func getItemDescription(isAPPUiOCloud bool, cluster, namespace string) string {
	if isAPPUiOCloud {
		return fmt.Sprintf("APPUiO Cloud - Cluster: %s / Namespace: %s", cluster, namespace)
	}
	return fmt.Sprintf("APPUiO Managed - Cluster: %s / Namespace: %s", cluster, namespace)
}

func getProductID(instances int, service string) string {
	sla := vshnv1.BestEffort
	if instances > 1 {
		sla = vshnv1.Guaranteed
	}

	// Construct productID: appcat-vshn-{service}-{sla}
	productID := fmt.Sprintf("appcat-vshn-%s-%s", service, sla)
	return productID
}
