package vshnpostgres

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/compat"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// AddPostgreSQLVersionCompatCheck flags and surfaces a version/revision
// incompatibility on the claim. Pure: only observes the matrix and writes
// desired composite status + a warning result.
func AddPostgreSQLVersionCompatCheck(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	revision := svc.Config.Data["revision"]
	comp.Status.CurrentRevision = revision

	res := compat.RunCompatCheck(ctx, svc, "postgresql",
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
