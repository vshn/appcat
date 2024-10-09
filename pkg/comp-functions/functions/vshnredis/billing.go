package vshnredis

import (
	"context"
	"fmt"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// AddServiceBillingLabel adds billingLabel to all the objects of a services that must be billed
func AddServiceBillingLabel(ctx context.Context, comp *v1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	return common.InjectBillingLabelToService(ctx, svc, comp)
}
