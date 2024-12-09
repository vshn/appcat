package maintenancecontroller

import (
	"context"
	"sync/atomic"

	managedupgradev1beta1 "github.com/appuio/openshift-upgrade-controller/api/v1beta1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

//+kubebuilder:rbac:groups=managedupgrade.appuio.io,resources=upgradejobs,verbs=get;list;watch

// MaintenanceReconciler handles the maintenance window detection for the current cluster.
// It watches the `UpgradeJob` CRs on the cluster and determines the current state of the maintenance.
type MaintenanceReconciler struct {
	maintenance atomic.Bool
	client.Client
}

// MaintenanceStatus returns the current state of the maintenance.
type MaintenanceStatus interface {
	IsMaintenanceRunning() bool
}

// Reconcile reconciles on OpenShift upgradeJob objects and tracks if it's currently running or not.
func (r *MaintenanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	l := log.FromContext(ctx).WithValues("namespace", req.Namespace, "upgradeJob", req.Name)

	inst := &managedupgradev1beta1.UpgradeJob{}
	err = r.Get(ctx, req.NamespacedName, inst)

	if apierrors.IsNotFound(err) || inst.DeletionTimestamp != nil {
		l.Info("UpgradeJob was deleted")
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	l.V(1).Info("Checking for any running maintenance")
	running, err := r.isAnyJobRunning(ctx, l)
	r.setMaintenance(running, l)
	return ctrl.Result{}, err
}

func (r *MaintenanceReconciler) isAnyJobRunning(ctx context.Context, l logr.Logger) (bool, error) {
	l.V(1).Info("Listing all upgradeJobs")

	jobs := &managedupgradev1beta1.UpgradeJobList{}
	err := r.List(ctx, jobs)
	if err != nil {
		return false, err
	}

	running := false
	for _, job := range jobs.Items {
		if r.jobState(job) == "active" {
			running = true
		}
	}

	return running, nil
}

// setMaintenance sets the status to the given boolean.
func (r *MaintenanceReconciler) setMaintenance(running bool, l logr.Logger) {
	l.Info("Setting maintenance state to", "state", running)
	r.maintenance.Store(running)
}

// IsMaintenanceRunning returns the current state of the maintenance.
func (r *MaintenanceReconciler) IsMaintenanceRunning() bool {
	return r.maintenance.Load()
}

func (r *MaintenanceReconciler) jobState(job managedupgradev1beta1.UpgradeJob) string {
	if apimeta.IsStatusConditionTrue(job.Status.Conditions, managedupgradev1beta1.UpgradeJobConditionSucceeded) {
		return "succeeded"
	} else if apimeta.IsStatusConditionTrue(job.Status.Conditions, managedupgradev1beta1.UpgradeJobConditionFailed) {
		return "failed"
	} else if apimeta.IsStatusConditionTrue(job.Status.Conditions, managedupgradev1beta1.UpgradeJobConditionStarted) {
		return "active"
	}
	return "pending"
}

// SetupWithManager sets up the controller with the Manager.
func (r *MaintenanceReconciler) SetupWithManager(mgr ctrl.Manager, serviceCluster *cluster.Cluster) error {
	sc := *serviceCluster

	// For external reconcile triggers we can't register a kind via `For()`
	// instead we have to name the reconciler something
	// usually the name is the lowercase of the kind
	// so that's what I've used here.
	return ctrl.NewControllerManagedBy(mgr).
		Named("upgradejob").
		// This is the magic sauce, it makes the reconciler reconcile on events happening on the serviceCluster
		WatchesRawSource(source.Kind(
			sc.GetCache(),
			&managedupgradev1beta1.UpgradeJob{},
			&handler.TypedEnqueueRequestForObject[*managedupgradev1beta1.UpgradeJob]{})).
		Complete(r)
}
