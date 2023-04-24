package controller

import (
	"context"
	"github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
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

type jsonpatch1 struct {
	Op    jsonOp `json:"op,omitempty"`
	Path  string `json:"path,omitempty"`
	Value int    `json:"value,-"`
}

var stackgresManagedKey = "stackgres.io~1managed-by-server-side-apply"

type xpostgreSQLDeletionProtectionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (p *xpostgreSQLDeletionProtectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
		log.Info("Scaling down the postgres instance to 0")
		err = p.deletingPostgresDB(ctx, inst)
	}

	return ctrl.Result{RequeueAfter: requeueTime}, err
}

func (p *xpostgreSQLDeletionProtectionReconciler) handleDeletionProtection(ctx context.Context, inst *vshnv1.XVSHNPostgreSQL) error {
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

func (p *xpostgreSQLDeletionProtectionReconciler) deletingPostgresDB(ctx context.Context, inst *vshnv1.XVSHNPostgreSQL) error {
	log := logging.FromContext(ctx, "namespace", inst.GetNamespace(), "instance", inst.GetName())

	log.V(1).Info("Getting postgres statefulset")
	o := &v1alpha1.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name: inst.Name + "-cluster",
		},
	}
	err := p.Delete(ctx, o)
	/*

		k := types.NamespacedName{
			Namespace: inst.Status.InstanceNamespace,
			Name:      inst.Name,
		}


		err := p.Get(ctx, k, ss)
		if err != nil {
			return fmt.Errorf("cannot get postgres statefulset %v", err)
		}

			var r int32 = 0
			ss.Spec.Replicas = &r
			// setting annotation to false does not work
			ss.Annotations = nil
			ss.Labels = nil
			ss.OwnerReferences = nil



		log.V(1).Info("Updating postgres statefulset")
		pa := []jsonpatch1{
			{
				Op:    opReplace,
				Path:  "/spec/replicas",
				Value: 0,
			},
		}
		a, err := json.Marshal(pa)
		b := client.RawPatch(types.JSONPatchType, a)
		err = p.Delete(ctx, ss, b)
		if err != nil {
			return fmt.Errorf("cannot update statefulset %v", err)
		}
	*/
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (p *xpostgreSQLDeletionProtectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.XVSHNPostgreSQL{}).
		Complete(p)
}
