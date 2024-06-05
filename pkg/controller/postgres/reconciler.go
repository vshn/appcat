package postgres

import (
	"context"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logging "sigs.k8s.io/controller-runtime/pkg/log"
)

//+kubebuilder:rbac:groups=kubernetes.crossplane.io,resources=objects,verbs=delete

// To run on newer OpenShift version, this RBAC permission is necessary.
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls/finalizers,verbs=get;list;patch;update;watch;create

type XPostgreSQLReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (p *XPostgreSQLReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logging.FromContext(ctx, "namespace", req.Namespace, "instance", req.Name)
	inst := &vshnv1.XVSHNPostgreSQL{}
	err := p.Get(ctx, req.NamespacedName, inst)

	if apierrors.IsNotFound(err) {
		log.Info("Instance deleted")
		return ctrl.Result{}, nil
	}

	err = p.handleDeletionProtection(ctx, inst)
	if err != nil {
		return ctrl.Result{}, err
	}
	requeueTime := getRequeueTime(ctx, inst, inst.GetDeletionTimestamp(), inst.Spec.Parameters.Backup.DeletionRetention)
	if inst.DeletionTimestamp != nil {
		log.Info("Deleting database")
		err = p.deletePostgresDB(ctx, inst)
		if err != nil {
			return ctrl.Result{RequeueAfter: requeueTime, Requeue: true}, err
		}
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: requeueTime,
	}, err
}

func (p *XPostgreSQLReconciler) handleDeletionProtection(ctx context.Context, inst *vshnv1.XVSHNPostgreSQL) error {
	log := logging.FromContext(ctx, "namespace", inst.GetNamespace(), "instance", inst.GetName())

	protectionEnabled := inst.Spec.Parameters.Backup.DeletionProtection
	retention := inst.Spec.Parameters.Backup.DeletionRetention
	baseObj := &vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      inst.Name,
			Namespace: inst.Namespace,
		},
	}

	patch, err := handle(ctx, inst, *protectionEnabled, retention)

	if err != nil {
		return errors.Wrap(err, "cannot return patch operation object")
	}

	overridePatch, err := getInstanceNamespaceOverride(ctx, inst, *protectionEnabled, p.Client)
	if err != nil {
		return errors.Wrap(err, "can't determine patch for namespace override")
	}

	if overridePatch != nil {
		patch = overridePatch
	}

	if patch != nil {

		errorFunc := func(err error) bool {
			return err != nil && !apierrors.IsNotFound(err)
		}

		// Unfortunately patches just return generic errors if you patch something that has been modified.
		// So we just retry a few times before actually logging and error.
		err := retry.OnError(retry.DefaultBackoff, errorFunc, func() error {
			log.V(1).Info("Trying to patch the object")
			return p.Patch(ctx, baseObj, patch)
		})

		if err != nil {
			return err
		}

	}

	return nil
}

func (p *XPostgreSQLReconciler) deletePostgresDB(ctx context.Context, inst *vshnv1.XVSHNPostgreSQL) error {
	log := logging.FromContext(ctx, "namespace", inst.GetNamespace(), "instance", inst.GetName())

	log.V(1).Info("Deleting sgcluster object")
	o := &xkube.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name: inst.Name + "-cluster",
		},
	}

	err := p.Delete(ctx, o)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (p *XPostgreSQLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.XVSHNPostgreSQL{}).
		Owns(&corev1.Namespace{}).
		Complete(p)
}
