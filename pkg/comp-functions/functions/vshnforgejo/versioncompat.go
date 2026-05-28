package vshnforgejo

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/compat"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
)

// AddForgejoVersionCompatCheck flags and surfaces a version/revision
// incompatibility on the claim.
func AddForgejoVersionCompatCheck(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	revision := svc.GetCompositionRevisionSelectorLabel(release.RevisionLabel)

	res := compat.RunCompatCheck(ctx, svc, "forgejo",
		comp.Spec.Parameters.Service.MajorVersion, revision,
		func(c vshnv1.Condition) {
			comp.Status.VersionCompatibilityConditions = compat.UpsertCondition(
				comp.Status.VersionCompatibilityConditions, c)
		})
	if res != nil {
		return res
	}

	if err := svc.SetDesiredCompositeStatus(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set composite status: %w", err))
	}
	return nil
}
