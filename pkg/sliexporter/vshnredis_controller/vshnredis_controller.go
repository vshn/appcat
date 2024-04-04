package vshnrediscontroller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	slireconciler "github.com/vshn/appcat/v4/pkg/sliexporter/sli_reconciler"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	vshnRedisServiceKey = "VSHNRedis"
)

type VSHNRedisReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ProbeManager       probeManager
	StartupGracePeriod time.Duration
	RedisDialer        func(service, name, namespace, organization, sla string, ha bool, opts redis.Options) (*probes.VSHNRedis, error)
}

type probeManager interface {
	slireconciler.ProbeManager
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

	nn := req.NamespacedName
	err = r.Get(ctx, nn, inst)

	reconciler := slireconciler.New(inst, l, r.ProbeManager, vshnRedisServiceKey, nn, err, r.StartupGracePeriod, r.getRedisProber)

	return reconciler.Reconcile(ctx)

}

func (r VSHNRedisReconciler) getRedisProber(ctx context.Context, obj slireconciler.Service) (prober probes.Prober, err error) {
	inst, ok := obj.(*vshnv1.XVSHNRedis)
	if !ok {
		return nil, fmt.Errorf("cannot start probe, object not a valid VSHNRedis")
	}

	credentials := v1.Secret{}

	err = r.Get(ctx, types.NamespacedName{
		Name:      inst.GetWriteConnectionSecretToReference().Name,
		Namespace: inst.GetWriteConnectionSecretToReference().Namespace,
	}, &credentials)

	if err != nil {
		return nil, err
	}

	ns := &corev1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: inst.ObjectMeta.Labels[slireconciler.ClaimNamespaceLabel]}, ns)
	if err != nil {
		return nil, err
	}

	org := ns.GetLabels()[utils.OrgLabelName]

	sla := inst.Spec.Parameters.Service.ServiceLevel

	tlsEnabled := inst.Spec.Parameters.TLS.TLSEnabled

	redisOptions := redis.Options{
		Addr:     string(credentials.Data["REDIS_HOST"]) + ":" + string(credentials.Data["REDIS_PORT"]),
		Username: string(credentials.Data["REDIS_USERNAME"]),
		Password: string(credentials.Data["REDIS_PASSWORD"]),
	}

	if tlsEnabled {
		tlsConfig := tls.Config{}
		certPair, err := tls.X509KeyPair(credentials.Data["tls.crt"], credentials.Data["tls.key"])
		if err != nil {
			return nil, err
		}
		tlsConfig = tls.Config{
			Certificates: []tls.Certificate{certPair},
			RootCAs:      x509.NewCertPool(),
		}

		tlsConfig.RootCAs.AppendCertsFromPEM(credentials.Data["ca.crt"])
		redisOptions.TLSConfig = &tlsConfig
	}

	prober, err = r.RedisDialer(vshnRedisServiceKey, inst.Name, inst.ObjectMeta.Labels[slireconciler.ClaimNamespaceLabel], org, string(sla), false, redisOptions)
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
