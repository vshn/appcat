package vshnmariadbcontroller

import (
	"context"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	slireconciler "github.com/vshn/appcat/v4/pkg/sliexporter/sli_reconciler"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	vshnMariadbServiceKey = "VSHNMariaDB"
	errNotReady           = fmt.Errorf("Resource is not yet ready")
)

type probeManager interface {
	slireconciler.ProbeManager
}

// VSHNMariaDBReconciler reconciles a VSHNMariaDB object
type VSHNMariaDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	//rootCAs *x509.CertPool

	ProbeManager       probeManager
	StartupGracePeriod time.Duration
	MariaDBDialer      func(service, name, claimNamespace, instanceNamespace, dsn, organization, serviceLevel, caCRT string, ha, TLSEnabled bool) (*probes.MariaDB, error)
	ScClient           client.Client
}

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnmariadbs,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnmariadbs/status,verbs=get
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnmariadbs,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnmariadbs/status,verbs=get

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile start or stops a prober for a VSHNMariaDB instance.
// Will only probe an instance once it is ready or after the StartupGracePeriod.
func (r *VSHNMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("namespace", req.Namespace, "instance", req.Name)

	inst := &vshnv1.XVSHNMariaDB{}

	reconciler := slireconciler.New(inst, l, r.ProbeManager, vshnMariadbServiceKey, req.NamespacedName, r.Client, r.StartupGracePeriod, r.fetchProberFor, r.ScClient)

	return reconciler.Reconcile(ctx)
}

func (r VSHNMariaDBReconciler) fetchProberFor(ctx context.Context, obj slireconciler.Service) (probes.Prober, error) {
	inst, ok := obj.(*vshnv1.XVSHNMariaDB)
	if !ok {
		return nil, fmt.Errorf("fetchProberFor: object is not a VSHNMariaDB")
	}

	credSecret := corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      inst.GetWriteConnectionSecretToReference().Name,
		Namespace: inst.GetWriteConnectionSecretToReference().Namespace,
	}, &credSecret)

	if err != nil {
		return nil, err
	}

	ready := r.areCredentialsAvailable(&credSecret)
	if !ready {
		return nil, errNotReady
	}

	claimNamespace := inst.ObjectMeta.Labels[slireconciler.ClaimNamespaceLabel]
	instanceNamespace := inst.Status.InstanceNamespace

	ns := &corev1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: claimNamespace}, ns)
	if err != nil {
		return nil, err
	}

	org := ns.GetLabels()[utils.OrgLabelName]
	if org == "" {
		org = "unknown"
	}
	sla := inst.Spec.Parameters.Service.ServiceLevel
	if sla == "" {
		sla = vshnv1.BestEffort
	}
	ha := inst.Spec.Parameters.Instances > 1

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", credSecret.Data["MARIADB_USERNAME"], credSecret.Data["MARIADB_PASSWORD"], credSecret.Data["MARIADB_HOST"], credSecret.Data["MARIADB_PORT"], "mysql")

	probe, err := r.MariaDBDialer(vshnMariadbServiceKey, inst.Name, claimNamespace, instanceNamespace, dsn, org, string(credSecret.Data["ca.crt"]), string(sla), ha, inst.Spec.Parameters.TLS.TLSEnabled)
	if err != nil {
		return nil, err
	}

	return probe, nil
}

func (r *VSHNMariaDBReconciler) areCredentialsAvailable(secret *corev1.Secret) bool {

	_, ok := secret.Data["MARIADB_HOST"]
	if !ok {
		return false
	}
	_, ok = secret.Data["MARIADB_PASSWORD"]
	if !ok {
		return false
	}
	_, ok = secret.Data["MARIADB_PORT"]
	if !ok {
		return false
	}
	_, ok = secret.Data["MARIADB_URL"]
	if !ok {
		return false
	}
	_, ok = secret.Data["MARIADB_USERNAME"]
	if !ok {
		return false
	}
	_, ok = secret.Data["ca.crt"]
	if !ok {
		return false
	}

	return ok
}

// SetupWithManager sets up the controller with the Manager.
func (r *VSHNMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.XVSHNMariaDB{}).
		Complete(r)
}
