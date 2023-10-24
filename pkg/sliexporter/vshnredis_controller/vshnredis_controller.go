package vshnrediscontroller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"time"

	"github.com/redis/go-redis/v9"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	vshnRedisServiceKey = "VSHNRedis"
	claimNamespaceLabel = "crossplane.io/claim-namespace"
	claimNameLabel      = "crossplane.io/claim-name"
)

type VSHNRedisReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ProbeManager       probeManager
	StartupGracePeriod time.Duration
	RedisDialer        func(service, name, namespace, organization, sla string, ha bool, opts redis.Options) (*probes.VSHNRedis, error)
}

type probeManager interface {
	StartProbe(p probes.Prober)
	StopProbe(p probes.ProbeInfo)
}

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnredis,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnredis/status,verbs=get
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnredis,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnredis/status,verbs=get

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *VSHNRedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	l := log.FromContext(ctx).WithValues("namespace", req.Namespace, "instance", req.Name)
	l.Info("Reconciling VSHNRedis")
	inst := &vshnv1.XVSHNRedis{}

	err = r.Get(ctx, req.NamespacedName, inst)

	if apierrors.IsNotFound(err) || inst.DeletionTimestamp != nil {
		l.Info("Stopping Probe")
		r.ProbeManager.StopProbe(probes.ProbeInfo{
			Service:   vshnRedisServiceKey,
			Name:      req.Name,
			Namespace: req.Namespace,
		})
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if inst.Spec.WriteConnectionSecretToRef.Name == "" {
		l.Info("No connection secret requested. Skipping.")
		return ctrl.Result{}, nil
	}

	probe, err := r.getRedisProber(ctx, inst)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) {
		l.WithValues("credentials", inst.Spec.WriteConnectionSecretToRef.Name, "error", err.Error()).
			Info("Failed to find credentials. Backing off")
		res.Requeue = true
		res.RequeueAfter = 30 * time.Second

		if time.Since(inst.GetCreationTimestamp().Time) < r.StartupGracePeriod {
			// Instance is starting up. Postpone probing until ready.
			return res, nil
		}

		// Create a pobe that will always fail
		probe, err = probes.NewFailingPostgreSQL(vshnRedisServiceKey, inst.Name, inst.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	l.Info("Starting Probe")
	r.ProbeManager.StartProbe(probe)
	return res, nil

}

func (r VSHNRedisReconciler) getRedisProber(ctx context.Context, inst *vshnv1.XVSHNRedis) (prober probes.Prober, err error) {
	instance := &vshnv1.VSHNRedis{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      inst.ObjectMeta.Labels[claimNameLabel],
		Namespace: inst.ObjectMeta.Labels[claimNamespaceLabel],
	}, instance)

	if err != nil {
		return nil, err
	}

	credentials := v1.Secret{}

	err = r.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.WriteConnectionSecretToRef.Name,
		Namespace: inst.ObjectMeta.Labels[claimNamespaceLabel],
	}, &credentials)

	if err != nil {
		return nil, err
	}

	ns := &corev1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: inst.ObjectMeta.Labels[claimNamespaceLabel]}, ns)
	if err != nil {
		return nil, err
	}

	org := ns.GetLabels()[utils.OrgLabelName]

	sla := inst.Spec.Parameters.Service.ServiceLevel

	certPair, err := tls.X509KeyPair(credentials.Data["tls.crt"], credentials.Data["tls.key"])
	if err != nil {
		return nil, err
	}
	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{certPair},
		RootCAs:      x509.NewCertPool(),
	}

	tlsConfig.RootCAs.AppendCertsFromPEM(credentials.Data["ca.crt"])

	prober, err = r.RedisDialer(vshnRedisServiceKey, inst.Name, inst.ObjectMeta.Labels[claimNamespaceLabel], org, string(sla), false, redis.Options{
		Addr:      string(credentials.Data["REDIS_HOST"]) + ":" + string(credentials.Data["REDIS_PORT"]),
		Username:  string(credentials.Data["REDIS_USERNAME"]),
		Password:  string(credentials.Data["REDIS_PASSWORD"]),
		TLSConfig: &tlsConfig,
	})
	if err != nil {
		return nil, err
	}

	return prober, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *VSHNRedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.XVSHNRedis{}).
		Complete(r)
}
