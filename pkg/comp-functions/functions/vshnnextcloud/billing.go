package vshnnextcloud

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// AddBilling enables billing for this service
func AddBilling(ctx context.Context, comp *v1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	if comp.Spec.Parameters.Service.Collabora.Enabled {
		return common.CreateBillingRecord(ctx, svc, comp, common.ServiceAddOns{
			Name:      "office",
			Instances: 1,
		})
	}

	return common.CreateBillingRecord(ctx, svc, comp)
}
