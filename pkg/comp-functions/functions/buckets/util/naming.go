package util

import (
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// GetBucketObjectName ensures that the right object name is chosen.
// Legacy buckets where created with wrong object names that break nested
// services.
// This logic returns the new naming scheme, if there's no existing bucket CR.
// If there's already a CR it will simply return it's name.
// The `obj` parameter should always be a deepCopy of the original, otherwise
// the pointer in the calling function will have all the fields populated. Which
// can lead to unexpected side effects.
func GetBucketObjectName(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, resName string, obj resource.Managed) string {

	err := svc.GetObservedComposedResource(obj, resName)
	if err != nil {
		return bucket.GetBucketName()
	}

	return obj.GetName()
}
