package common

import (
	"context"

	"github.com/vshn/appcat/v4/apis/metadata"
	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/common/utils"
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

		orgAdded := false
		objectMeta := &metadata.MetadataOnlyObject{}

		err = iof.Desired.GetComposite(ctx, objectMeta)
		if err != nil {
			return runtime.NewFatalErr(ctx, "cannot get composite meta", err)
		}

		if value, ok := ns.GetLabels()[utils.OrgLabelName]; !ok || value == "" {
			objectMeta := &metadata.MetadataOnlyObject{}

			err := iof.Desired.GetComposite(ctx, objectMeta)
			if err != nil {
				return runtime.NewFatalErr(ctx, "cannot get composite meta", err)
			}

			if ns.Labels == nil {
				ns.Labels = map[string]string{}
			}

			ns.Labels[utils.OrgLabelName] = objectMeta.GetLabels()[utils.OrgLabelName]
			orgAdded = true
		}

		s, err := utils.FetchSidecarsFromConfig(ctx, iof)
		if err != nil {
			s = &utils.Sidecars{}
		}

		// We only act if either the quotas were missing or the organization label is not on the
		// namespace. Otherwise we ignore updates. This is to prevent any unwanted overwriting.
		if quotas.AddInitalNamespaceQuotas(ctx, iof, ns, s, objectMeta.TypeMeta.Kind) || orgAdded {
			err = iof.Desired.PutIntoObject(ctx, ns, namespaceKon)
			if err != nil {
				return runtime.NewFatalErr(ctx, "cannot save namespace quotas", err)
			}
		}

		return runtime.NewNormal()
	}
}
