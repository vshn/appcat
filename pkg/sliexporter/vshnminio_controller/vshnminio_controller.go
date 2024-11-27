package vshnminiocontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	slireconciler "github.com/vshn/appcat/v4/pkg/sliexporter/sli_reconciler"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/minio/minio-go/v7/pkg/credentials"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	vshnMinioServiceKey = "VSHNMinio"
	SLIBucketName       = "vshn-test-bucket-for-sli"
)

type VSHNMinioReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ProbeManager       probeManager
	StartupGracePeriod time.Duration
	MinioDialer        func(service, name, namespace, organization, sla, endpointURL string, ha bool, opts minio.Options) (*probes.VSHNMinio, error)
	ScClient           client.Client
}

type probeManager interface {
	slireconciler.ProbeManager
}

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnminios,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnminios/status,verbs=get
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnminios,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnminios/status,verbs=get

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *VSHNMinioReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	l := log.FromContext(ctx).WithValues("namespace", req.Namespace, "instance", req.Name)
	l.Info("Reconciling XVSHNMinio")

	inst := &vshnv1.XVSHNMinio{}

	reconciler := slireconciler.New(inst, l, r.ProbeManager, vshnMinioServiceKey, req.NamespacedName, r.Client, r.StartupGracePeriod, r.getMinioProber, r.ScClient)

	return reconciler.Reconcile(ctx)

}

func (r VSHNMinioReconciler) getMinioProber(ctx context.Context, obj slireconciler.Service) (prober probes.Prober, err error) {
	inst, ok := obj.(*vshnv1.XVSHNMinio)
	if !ok {
		return nil, fmt.Errorf("cannot start probe, object not a valid VSHNRedis")
	}

	l := log.FromContext(ctx).WithValues("namespace", inst.ObjectMeta.Labels[slireconciler.ClaimNamespaceLabel], "instance", inst.ObjectMeta.Labels[slireconciler.ClaimNameLabel])

	creds := corev1.Secret{}

	l.Info("looking for secret:"+SLIBucketName, "namespace", "vshn-minio-"+inst.Name)

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
		inst.ObjectMeta.Labels[slireconciler.ClaimNamespaceLabel],
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
