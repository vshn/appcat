package vshnpostgres

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func DeployPostgreSQL(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
	l := svc.Log

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		err = fmt.Errorf("cannot get observed composite: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, "postgresql", "namespace-conditions", svc)
	if err != nil {
		err = fmt.Errorf("cannot bootstrap instance namespace: %w", err)
		return runtime.NewFatalResult(err)
	}

	return nil

}
