package controller

import (
	"context"
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	logging "sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	vshnv1 "github.com/vshn/component-appcat/apis/vshn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type XPostgreSQLDeletionProtectionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (p *XPostgreSQLDeletionProtectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = context.WithValue(ctx, "now", time.Now())

	log := logging.FromContext(ctx, "namespace", req.Namespace, "instance", req.Name)
	inst := &vshnv1.XVSHNPostgreSQL{}
	err := p.Get(ctx, req.NamespacedName, inst)

	if apierrors.IsNotFound(err) {
		log.Info("Instance deleted")
		return ctrl.Result{}, nil
	}

	err = p.handleDeletionProtection(ctx, inst)
	requeueTime := getRequeueTime(ctx, inst, inst.GetDeletionTimestamp(), inst.Spec.Parameters.Backup.Retention)
	if err != nil {
		return ctrl.Result{RequeueAfter: requeueTime}, err
	}

	if inst.DeletionTimestamp != nil {
		log.Info("Deleting database")
		err = p.deletingPostgresDB(ctx, inst)
	}

	return ctrl.Result{RequeueAfter: requeueTime}, err
}

func (p *XPostgreSQLDeletionProtectionReconciler) handleDeletionProtection(ctx context.Context, inst *vshnv1.XVSHNPostgreSQL) error {
	protectionEnabled := inst.Spec.Parameters.Backup.DeletionProtection
	retention := inst.Spec.Parameters.Backup.DeletionRetention
	baseObj := &vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      inst.Name,
			Namespace: inst.Namespace,
		},
	}

	patch, err := handle(ctx, inst, protectionEnabled, retention)

	if err != nil {
		return errors.Wrap(err, "cannot return patch operation object")
	}

	if patch != nil {
		if err = p.Patch(ctx, baseObj, patch); err != nil && !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "could not patch object")
		}
	}

	return nil
}

func (p *XPostgreSQLDeletionProtectionReconciler) deletingPostgresDB(ctx context.Context, inst *vshnv1.XVSHNPostgreSQL) error {
	log := logging.FromContext(ctx, "namespace", inst.GetNamespace(), "instance", inst.GetName())

	log.V(1).Info("Deleting sgcluster object")
	o := &xkube.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name: inst.Name + "-cluster",
		},
	}

	return p.Delete(ctx, o)
}

// SetupWithManager sets up the controller with the Manager.
func (p *XPostgreSQLDeletionProtectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.XVSHNPostgreSQL{}).
		Complete(p)
}
