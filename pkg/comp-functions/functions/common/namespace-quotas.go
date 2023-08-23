package common

import (
	"context"

	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

// AddInitialNamespaceQuotas will add the default quotas to a namespace if they are not yet set.
// This function takes the name of the namespace resource as it appears in the functionIO, it then returns the actual
// function that implements the composition function step.
func AddInitialNamespaceQuotas(namespaceKon string) func(context.Context, *runtime.Runtime) runtime.Result {
	return func(ctx context.Context, iof *runtime.Runtime) runtime.Result {
		if !iof.GetBoolFromCompositionConfig("quotasEnabled") {
			return runtime.NewNormal()
		}

		ns := &corev1.Namespace{}

		err := iof.Observed.GetFromObject(ctx, ns, namespaceKon)
		if err != nil {
			if err == runtime.ErrNotFound {
				err = iof.Desired.GetFromObject(ctx, ns, namespaceKon)
				if err != nil {
					return runtime.NewWarning(ctx, "cannot get namespace: "+err.Error())
				}
			} else {
				return runtime.NewWarning(ctx, "cannot get namespace: "+err.Error())
			}
		}

		quotas.AddInitalNamespaceQuotas(ns)

		err = iof.Desired.PutIntoObject(ctx, ns, namespaceKon)
		if err != nil {
			return runtime.NewFatalErr(ctx, "cannot save namespace quotas", err)
		}

		return runtime.NewNormal()
	}
}
