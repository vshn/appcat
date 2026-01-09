package vshnkeycloak

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
func AddBilling(ctx context.Context, comp *v1.VSHNKeycloak, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	// Keep the existing Prometheus-based billing
	prometheusResult := common.CreateBillingRecord(ctx, svc, comp)
	if prometheusResult != nil && prometheusResult.Severity != xfnproto.Severity_SEVERITY_NORMAL {
		return prometheusResult
	}

	// Add BillingService CR-based billing
	billingServiceResult := common.CreateOrUpdateBillingServiceWithOptions(ctx, svc, comp, common.BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})

	if billingServiceResult != nil && billingServiceResult.Severity != xfnproto.Severity_SEVERITY_NORMAL {
		return billingServiceResult
	}

	return runtime.NewNormalResult(fmt.Sprintf("Billing enabled for instance %s", comp.GetName()))
}
