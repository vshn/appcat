package common

import (
	xpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	apix "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UsageOfBy helps with ordered deletions.
// Sometimes there are objects that are essential for porviders to work.
// For example provider-sql needs secrets to connect to instances.
// During the deletion it's not guaranteed that the secret gets deleted after
// the managed resource that the provider manages.
// This will essentially make it deadlock, as the managed resource will still
// contain a finalizer which blocks the deletion.
// See: https://docs.crossplane.io/latest/concepts/usages/#usage-for-deletion-ordering
//
// Of is the managed resource that should be protected
// By is the managed resource which should block the deletion. As long as it exists
// the deletion of "Of" will be denied.
func UsageOfBy(of, by xpresource.Managed, svc *runtime.ServiceRuntime) error {
	name := by.GetName() + "-usage"
	ofAPIVersion, ofKind := of.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	byAPIVersion, byKind := by.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()

	usage := &apix.Usage{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apix.UsageSpec{
			Of: apix.Resource{
				APIVersion: ofAPIVersion,
				Kind:       ofKind,
				ResourceRef: &apix.ResourceRef{
					Name: of.GetName(),
				},
			},
			By: &apix.Resource{
				APIVersion: byAPIVersion,
				Kind:       byKind,
				ResourceRef: &apix.ResourceRef{
					Name: by.GetName(),
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(usage, name)
}
