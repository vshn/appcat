package webhooks

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=*,verbs=get;list;watch

const (
	ProtectionOverrideLabel = "appcat.vshn.io/webhook-allowdeletion"
)

var (
	errNoOwnerRefAnnotation = fmt.Errorf("ownerReference annotation not specified")
)

type compositeInfo struct {
	Exists bool
	Name   string
}

// DeletionProtectionInfo provides information about the composite's deletion
// protection state.
type DeletionProtectionInfo interface {
	IsDeletionProtected() bool
}

// checkManagedObject will find the highest composite for any object that is deployed via Crossplane or provider-kubernetes.
// For example: XObjectBucket, Release, Namespace, etc.
// Anything that either contains an owner reference to a composite or an owner annotation.
func checkManagedObject(ctx context.Context, obj client.Object, c client.Client, l logr.Logger) (compositeInfo, error) {

	ownerKind, ok := obj.GetLabels()[runtime.OwnerKindAnnotation]
	if !ok || ownerKind == "" {
		l.Info(runtime.OwnerKindAnnotation + " label not set, skipping evaluation")
		return compositeInfo{Exists: false}, nil
	}

	ownerVersion, ok := obj.GetLabels()[runtime.OwnerVersionAnnotation]
	if !ok || ownerVersion == "" {
		l.Info(runtime.OwnerVersionAnnotation + " label not found, skipping evaluation")
		return compositeInfo{Exists: false}, nil
	}

	onwerGroup, ok := obj.GetLabels()[runtime.OwnerGroupAnnotation]
	if !ok || onwerGroup == "" {
		l.Info(runtime.OwnerGroupAnnotation + " label not found, skipping evaluation")
		return compositeInfo{Exists: false}, nil
	}

	ownerName, ok := obj.GetLabels()["crossplane.io/composite"]
	if !ok || ownerName == "" {
		l.Info("crossplane.io/composite label not found, skipping evaluation")
		return compositeInfo{Exists: false}, nil
	}

	gvk := schema.GroupVersionKind{
		Kind:    ownerKind,
		Version: ownerVersion,
		Group:   onwerGroup,
	}

	rcomp, err := c.Scheme().New(gvk)
	if err != nil {
		// We inform the user if the scheme doesn't know about the composite.
		if k8sruntime.IsNotRegisteredError(err) {
			return compositeInfo{Exists: false, Name: ownerName}, err
		}
		// If we can't parse the gvk, we don't block deletions, it can lead to
		// deadlocks otherwise...
		return compositeInfo{Exists: false, Name: ownerName}, nil
	}

	comp, ok := rcomp.(client.Object)
	if !ok {
		return compositeInfo{Exists: false, Name: ownerName}, fmt.Errorf("object is not a valid client object: %s", ownerName)
	}

	err = c.Get(ctx, client.ObjectKey{Name: ownerName}, comp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return compositeInfo{Exists: false, Name: ownerName}, nil
		}
		// Here we can't determine if it still exists or not, so we go the
		// save route and assume it does.
		return compositeInfo{Exists: true, Name: ownerName}, fmt.Errorf("cannot get composite: %w", err)
	}

	// Check if the composite is already being deleted, then we disengage the
	// protection.
	if comp.GetDeletionTimestamp() != nil {
		return compositeInfo{Exists: false, Name: ownerName}, nil
	}

	return compositeInfo{Exists: isDeletionProtected(obj), Name: ownerName}, nil
}

// checkUnmanagedObject tries to get the composite information about objects that are not directly managed by Crossplane.
// It checks if the namespace it belongs to is managed by a composite, if that's the case it uses the same logic to
// determine the state of the deletion protection.
// Such objects would be: any helm generated object, pvcs for sts and any other 3rd party managed objects.
func checkUnmanagedObject(ctx context.Context, obj client.Object, c client.Client, l logr.Logger) (compositeInfo, error) {
	namespace := &corev1.Namespace{}

	err := c.Get(ctx, client.ObjectKey{Name: obj.GetNamespace()}, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return compositeInfo{Exists: false}, nil
		}
		return compositeInfo{Exists: false}, err
	}

	return checkManagedObject(ctx, namespace, c, l)

}

// isDeletionProtected checks the protection override label
// and determines if the object should be protected or not.
// If the label is not set, the protection is active.
// If the label contains any other value than "true", then the
// protection is active.
func isDeletionProtected(obj client.Object) bool {
	val, ok := obj.GetLabels()[ProtectionOverrideLabel]
	// If the label is not there, the protection is enabled.
	if !ok {
		return true
	}
	// If the value is "true" the protection is disabled.
	return !(val == "true")
}
