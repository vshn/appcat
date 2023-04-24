package controller

import (
	"context"
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

type postgreSQLDeletionProtectionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (p *postgreSQLDeletionProtectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = context.WithValue(ctx, currentTimeKey, time.Now())

	log := logging.FromContext(ctx, "namespace", req.Namespace, "instance", req.Name)
	inst := &vshnv1.VSHNPostgreSQL{}
	err := p.Get(ctx, req.NamespacedName, inst)

	if apierrors.IsNotFound(err) {
		log.Info("Instance deleted")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{
		RequeueAfter: getRequeueTime(ctx, inst, inst.GetDeletionTimestamp(), inst.Spec.Parameters.Backup.Retention),
	}, p.handleDeletionProtection(ctx, inst)
}

func (p *postgreSQLDeletionProtectionReconciler) handleDeletionProtection(ctx context.Context, inst *vshnv1.VSHNPostgreSQL) error {
	protectionEnabled := inst.Spec.Parameters.Backup.DeletionProtection
	retention := inst.Spec.Parameters.Backup.DeletionRetention
	baseObj := &vshnv1.VSHNPostgreSQL{
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

// SetupWithManager sets up the controller with the Manager.
func (p *postgreSQLDeletionProtectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.VSHNPostgreSQL{}).
		Complete(p)
}
