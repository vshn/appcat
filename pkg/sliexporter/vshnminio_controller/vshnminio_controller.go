package vshnminiocontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/minio/minio-go/v7/pkg/credentials"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	vshnMinioServiceKey = "VSHNMinio"
	claimNamespaceLabel = "crossplane.io/claim-namespace"
	claimNameLabel      = "crossplane.io/claim-name"
	SLIBucketName       = "vshn-test-bucket-for-sli"
)

type VSHNMinioReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ProbeManager       probeManager
	StartupGracePeriod time.Duration
	MinioDialer        func(service, name, namespace, organization, sla, endpointURL string, ha bool, opts minio.Options) (*probes.VSHNMinio, error)
}

type probeManager interface {
	StartProbe(p probes.Prober)
	StopProbe(p probes.ProbeInfo)
}

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnminios,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnminios/status,verbs=get
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnminios,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnminios/status,verbs=get

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *VSHNMinioReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	l := log.FromContext(ctx).WithValues("namespace", req.Namespace, "instance", req.Name)
	l.Info("Reconciling VSHNRedis")

	inst := &vshnv1.XVSHNMinio{}
	err = r.Get(ctx, req.NamespacedName, inst)

	if apierrors.IsNotFound(err) || inst.DeletionTimestamp != nil {
		l.Info("Stopping Probe")
		r.ProbeManager.StopProbe(probes.ProbeInfo{
			Service:   vshnMinioServiceKey,
			Name:      req.Name,
			Namespace: req.Namespace,
		})
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if inst.Spec.WriteConnectionSecretToReference == nil || inst.Spec.WriteConnectionSecretToReference.Name == "" {
		l.Info("No connection secret requested. Skipping.")
		return ctrl.Result{}, nil
	}

	probe, err := r.getMinioProber(ctx, inst)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) {
		l.WithValues("credentials", inst.Spec.WriteConnectionSecretToReference.Name, "error", err.Error()).
			Info("Failed to find credentials. Backing off")
		res.Requeue = true
		res.RequeueAfter = 30 * time.Second

		if time.Since(inst.GetCreationTimestamp().Time) < r.StartupGracePeriod {
			// Instance is starting up. Postpone probing until ready.
			return res, nil
		}
	}

	l.Info("Starting Probe")

	r.ProbeManager.StartProbe(probe)
	return res, nil

}

func (r VSHNMinioReconciler) getMinioProber(ctx context.Context, inst *vshnv1.XVSHNMinio) (prober probes.Prober, err error) {
	l := log.FromContext(ctx).WithValues("namespace", inst.ObjectMeta.Labels[claimNamespaceLabel], "instance", inst.ObjectMeta.Labels[claimNameLabel])

	creds := corev1.Secret{}

	fmt.Println("looking for secret: ", SLIBucketName, " in namespace: ", "vshn-minio-"+inst.Name)

	err = r.Get(ctx, types.NamespacedName{
		Name:      SLIBucketName,
		Namespace: "vshn-minio-" + inst.Name,
	}, &creds)

	if err != nil {
		return nil, err
	}

	ha := false
	sla := vshnv1.BestEffort
	if inst.Spec.Parameters.Instances >= 4 {
		ha = true
	}

	prober, err = r.MinioDialer(
		vshnMinioServiceKey,
		inst.Name,
		inst.ObjectMeta.Labels[claimNamespaceLabel],
		inst.GetLabels()[utils.OrgLabelName],
		string(sla),
		string(creds.Data["ENDPOINT"]),
		ha,
		minio.Options{
			Creds:  credentials.NewStaticV4(string(creds.Data["AWS_ACCESS_KEY_ID"]), string(creds.Data["AWS_SECRET_ACCESS_KEY"]), ""),
			Secure: false,
		})
	if err != nil {
		l.Error(err, "Can't create Minio Dialer")
		return nil, err
	}

	return prober, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *VSHNMinioReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.XVSHNMinio{}).
		Complete(r)
}
