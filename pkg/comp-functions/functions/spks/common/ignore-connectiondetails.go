package common

import (
	"context"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IgnoreConnectionDetailFix will loop over all desired resources and
// set the annotation to skip the re-writing of the connection details.
// This is necessary, or otherwise we end up with a lot of additional objects that
// are useless. As SPKS' connectionDetail management already works as intended
// with a split cluster configuration.
func IgnoreConnectionDetailFix[T client.Object](ctx context.Context, obj T, svc *runtime.ServiceRuntime) *fnproto.Result {

	desired := svc.GetAllDesired()

	for name, res := range desired {
		if res.Resource.GetWriteConnectionSecretToReference() != nil {
			annotations := res.Resource.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[runtime.IgnoreConnectionDetailsAnnotation] = "true"
			res.Resource.SetAnnotations(annotations)
			desired[name] = res
		}
	}

	svc.SetAllDesired(desired)

	return nil
}
