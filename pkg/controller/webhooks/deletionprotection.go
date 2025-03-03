package webhooks

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=*,verbs=get;list;watch

const (
	ProtectionOverrideLabel = "appcat.vshn.io/webhook-allowdeletion"
	protectedMessage        = "%s is part of a VSHN AppCat service and protected from deletions. Either Delete the the claim for composite %s or set this label on the object: 'appcat.vshn.io/webhook-allowdeletion: \"true\"'"
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
func checkManagedObject(ctx context.Context, obj client.Object, c client.Client, cpClient client.Client, l logr.Logger) (compositeInfo, error) {

	if cpClient == nil {
		cpClient = c
	}

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

	err = cpClient.Get(ctx, client.ObjectKey{Name: ownerName}, comp)
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
func checkUnmanagedObject(ctx context.Context, obj client.Object, c client.Client, cpClient client.Client, l logr.Logger) (compositeInfo, error) {
	namespace := &corev1.Namespace{}

	// we need to check namespaces against the local service cluster, otherwise they are never actually found
	err := c.Get(ctx, client.ObjectKey{Name: obj.GetNamespace()}, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return compositeInfo{Exists: false}, nil
		}
		return compositeInfo{Exists: false}, err
	}

	compInfo, err := checkManagedObject(ctx, namespace, c, cpClient, l)
	if err != nil {
		return compositeInfo{}, err
	}

	compInfo.Exists = compInfo.Exists && isDeletionProtected(obj)

	return compInfo, nil

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

func getControlPlaneClient(mgr ctrl.Manager) (client.Client, error) {

	cpClient := mgr.GetClient()
	if viper.IsSet("CONTROL_PLANE_KUBECONFIG") {
		kubeconfigPath := viper.GetString("CONTROL_PLANE_KUBECONFIG")
		kubeconfig, err := os.ReadFile(kubeconfigPath)
		if err != nil {
			return cpClient, err
		}
		config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
		if err != nil {
			return cpClient, err
		}
		cpClient, err = client.New(config, client.Options{
			Scheme: mgr.GetScheme(),
		})
		if err != nil {
			return cpClient, err
		}
	}
	return cpClient, nil
}
