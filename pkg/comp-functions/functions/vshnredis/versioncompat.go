package vshnredis

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/compat"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
)

// AddRedisVersionCompatCheck flags and surfaces a version/revision
// incompatibility on the claim. Pure: only observes the matrix and writes
// desired composite status + a warning result.
func AddRedisVersionCompatCheck(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	revision := svc.GetCompositionRevisionSelectorLabel(release.RevisionLabel)

	res := compat.RunCompatCheck(ctx, svc, "redis",
		comp.Spec.Parameters.Service.Version, revision,
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
