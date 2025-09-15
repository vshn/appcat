package vshnpostgrescnpg

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// EnsureObjectBucketLabels just gets the bucket present from the PnT part and adds it again to the
// desired state. This ensures that the correct labels are injected.
func EnsureObjectBucketLabels(ctx context.Context, comp *vshnv1.VSHNPostgreSQLCNPG, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	bucket := &appcatv1.XObjectBucket{}

	err = svc.GetDesiredComposedResourceByName(bucket, "pg-bucket")
	if err != nil {
		return runtime.NewWarningResult("cannot get xobjectbucket")
	}

	labels := map[string]string{
		"appcat.vshn.io/ignore-provider-config": "true",
	}

	svc.AddLabels(bucket, labels)

	err = svc.SetDesiredComposedResourceWithName(bucket, "pg-bucket")
	if err != nil {
		return runtime.NewWarningResult("cannot add xobjectbucket to desired map")
	}

	return nil
}
