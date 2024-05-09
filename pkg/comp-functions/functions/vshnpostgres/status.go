package vshnpostgres

import (
	"context"
	"fmt"
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func UpdateStatus(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	np := &xkube.Object{}
	err = svc.GetObservedComposedResource(np, comp.GetName()+"-netpol")
	if err != nil && err == runtime.ErrNotFound {
		return runtime.NewNormalResult(fmt.Sprintf("Object %s missing. Skipping status update", comp.GetName()+"-netpol"))
	} else if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get network policy: %w", err))
	}

	log.Info("Update network policy conditions")
	comp.Status.NetworkPolicyConditions = np.Status.Conditions
	err = svc.SetDesiredCompositeStatus(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get update composite status: %w", err))
	}
	return nil
}
