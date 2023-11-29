package common

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/vshn/appcat/v4/apis/metadata"
	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

// AddInitialNamespaceQuotas will add the default quotas to a namespace if they are not yet set.
// This function takes the name of the namespace resource as it appears in the functionIO, it then returns the actual
// function that implements the composition function step.
func AddInitialNamespaceQuotas(namespaceKon string) func(context.Context, *runtime.ServiceRuntime) *xfnproto.Result {
	return func(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
		if !svc.GetBoolFromCompositionConfig("quotasEnabled") {
			return nil
		}

		ns := &corev1.Namespace{}

		err := svc.GetObservedKubeObject(ns, namespaceKon)
		if err != nil {
			if err == runtime.ErrNotFound {
				err = svc.GetDesiredKubeObject(ns, namespaceKon)
				if err != nil {
					return runtime.NewFatalResult(fmt.Errorf("cannot get namespace: %w", err))
				}
				// Make sure we don't touch this, if there's no name in the namespace.
				if ns.GetName() == "" {
					return runtime.NewWarningResult("namespace doesn't yet have a name")
				}
			} else {
				return runtime.NewFatalResult(fmt.Errorf("cannot get namespace: %w", err))
			}
		}

		orgAdded := false
		objectMeta := &metadata.MetadataOnlyObject{}

		err = svc.GetObservedComposite(objectMeta)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot get composite meta: %w", err))
		}

		if value, ok := ns.GetLabels()[utils.OrgLabelName]; !ok || value == "" {
			objectMeta := &metadata.MetadataOnlyObject{}

			err := svc.GetObservedComposite(objectMeta)
			if err != nil {
				return runtime.NewFatalResult(fmt.Errorf("cannot get composite meta: %w", err))
			}

			if ns.Labels == nil {
				ns.Labels = map[string]string{}
			}

			ns.Labels[utils.OrgLabelName] = objectMeta.GetLabels()[utils.OrgLabelName]
			orgAdded = true
		}

		s, err := utils.FetchSidecarsFromConfig(ctx, svc)
		if err != nil {
			s = &utils.Sidecars{}
		}

		// We only act if either the quotas were missing or the organization label is not on the
		// namespace. Otherwise we ignore updates. This is to prevent any unwanted overwriting.
		if quotas.AddInitalNamespaceQuotas(ctx, ns, s, objectMeta.TypeMeta.Kind) || orgAdded {
			err = svc.SetDesiredKubeObjectWithName(ns, ns.GetName(), namespaceKon)
			if err != nil {
				return runtime.NewFatalResult(fmt.Errorf("cannot save namespace quotas: %w", err))
			}
		}

		return nil
	}
}
