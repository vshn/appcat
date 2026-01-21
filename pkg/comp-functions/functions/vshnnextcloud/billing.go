package vshnnextcloud

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// AddBilling enables billing for this service
// It runs both the legacy Prometheus-based billing and the BillingService CR-based billing
func AddBilling(ctx context.Context, comp *v1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	// Keep the existing Prometheus-based billing (with office addon if Collabora is enabled)
	var prometheusResult *xfnproto.Result
	if comp.Spec.Parameters.Service.Collabora.Enabled {
		prometheusResult = common.CreateBillingRecord(ctx, svc, comp, common.ServiceAddOns{
			Name:      "office",
			Instances: 1,
		})
	} else {
		prometheusResult = common.CreateBillingRecord(ctx, svc, comp)
	}

	if prometheusResult != nil && prometheusResult.Severity != xfnproto.Severity_SEVERITY_NORMAL {
		return prometheusResult
	}

	// Add BillingService CR-based billing with Collabora add-on if enabled
	billingOpts := common.BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	}

	if comp.Spec.Parameters.Service.Collabora.Enabled {
		billingOpts.Items = []v1.ItemSpec{
			{
				ProductID: "appcat-vshn-nextcloud-office-besteffort",
				Value:     "1",
			},
		}
	}

	billingServiceResult := common.CreateOrUpdateBillingServiceWithOptions(ctx, svc, comp, billingOpts)

	if billingServiceResult != nil && billingServiceResult.Severity != xfnproto.Severity_SEVERITY_NORMAL {
		return billingServiceResult
	}

	return runtime.NewNormalResult(fmt.Sprintf("Billing enabled for instance %s", comp.GetName()))
}
