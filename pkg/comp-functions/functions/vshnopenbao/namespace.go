package vshnopenbao

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

const namespaceSuffix = "-ns"

func BootstrapNamespace(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	serviceName := comp.GetName()

	_ = common.DisableBilling(comp.GetInstanceNamespace(), svc)

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	svc.Log.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, serviceName, serviceName+namespaceSuffix, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot bootstrap instance namespace: %s", err))
	}

	return nil
}
