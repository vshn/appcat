package vshnpostgrescnpg

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// AddBilling enables billing for this service
// It runs both the legacy Prometheus-based billing and the new BillingService CR-based billing
func AddBilling(ctx context.Context, comp *v1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	// Keep the existing Prometheus-based billing
	prometheusResult := common.CreateBillingRecord(ctx, svc, comp)
	if prometheusResult != nil && prometheusResult.Severity == xfnproto.Severity_SEVERITY_FATAL {
		return prometheusResult
	}
	// Add new BillingService CR-based billing
	billingServiceResult := common.CreateOrUpdateBillingService(ctx, svc, comp)

	if billingServiceResult != nil && billingServiceResult.Severity == xfnproto.Severity_SEVERITY_FATAL {
		return billingServiceResult
	}

	if billingServiceResult != nil {
		return runtime.NewNormalResult(fmt.Sprintf("Billing enabled (Prometheus + BillingService) for instance %s", comp.GetName()))
	}

	return prometheusResult
}
