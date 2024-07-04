package vshnpostgres

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

// DelayClusterDeployment adds the dependencies on the sgcluster object, so that the provider-kubernetes doesn't loop anymore.
// The actual cluster object will only be deployed once the dependencies are deployed, synced and ready.
func DelayClusterDeployment(_ context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)

	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite: %w", err))
	}

	clusterObject := &xkube.Object{}
	err = svc.GetObservedComposedResource(clusterObject, "cluster")
	if err == nil {
		// We're done here, the sgcluster has been applied.
		return nil
	} else if err != runtime.ErrNotFound {
		return runtime.NewWarningResult(fmt.Errorf("Cannot get cluster: %w", err).Error())
	}

	desiredCluster := &stackgresv1.SGCluster{}
	err = svc.GetDesiredKubeObject(desiredCluster, "cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot get desired cluster: %w", err).Error())
	}

	svc.DeleteDesiredCompososedResource("cluster")

	if !kubeObjectSyncedAndReady("profile", svc) {
		return runtime.NewWarningResult("sgProfile is not yet ready, skipping creation of cluster")
	}

	if !kubeObjectSyncedAndReady("sg-backup", svc) {
		return runtime.NewWarningResult("SGObjectStorage is not yet ready, skipping creation of cluster")
	}

	if !kubeObjectSyncedAndReady("pg-conf", svc) {
		return runtime.NewWarningResult("SGPostgresConfig is not yet ready, skipping creation of cluster")
	}

	err = svc.SetDesiredKubeObjectWithName(desiredCluster, comp.GetName()+"-cluster", "cluster")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot set SgCluster object: %w", err))
	}

	return nil
}

func kubeObjectSyncedAndReady(name string, svc *runtime.ServiceRuntime) bool {

	obj := &xkube.Object{}
	err := svc.GetObservedComposedResource(obj, name)
	if err != nil {
		return false
	}
	ready := obj.GetCondition(xpv1.TypeReady)
	if ready.Status == corev1.ConditionFalse {
		return false
	}

	synced := obj.GetCondition(xpv1.TypeSynced)

	return synced.Status == corev1.ConditionTrue
}
