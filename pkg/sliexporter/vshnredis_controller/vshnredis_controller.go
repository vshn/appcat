package vshnrediscontroller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"time"

	"github.com/redis/go-redis/v9"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var vshnRedisServiceKey = "VSHNRedis"

type VSHNRedisReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ProbeManager       probeManager
	StartupGracePeriod time.Duration
	RedisDialer        func(service, name, namespace, organization string, opts redis.Options) (*probes.VSHNRedis, error)
}

type probeManager interface {
	StartProbe(p probes.Prober)
	StopProbe(p probes.ProbeInfo)
}

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnredis,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnredis/status,verbs=get

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *VSHNRedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	l := log.FromContext(ctx).WithValues("namespace", req.Namespace, "instance", req.Name)
	l.Info("Reconciling VSHNRedis")
	inst := &vshnv1.VSHNRedis{}
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

func (r VSHNRedisReconciler) getRedisProber(ctx context.Context, inst *vshnv1.VSHNRedis) (prober probes.Prober, err error) {
	credSecret := corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      inst.Spec.WriteConnectionSecretToRef.Name,
		Namespace: inst.Namespace,
	}, &credSecret)

	if err != nil {
		return nil, err
	}

	ns := &corev1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: inst.Namespace}, ns)
	if err != nil {
		return nil, err
	}

	org := ns.GetLabels()["appuio.io/organization"]

	certPair, err := tls.X509KeyPair(credSecret.Data["tls.crt"], credSecret.Data["tls.key"])
	if err != nil {
		return nil, err
	}
	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{certPair},
		RootCAs:      x509.NewCertPool(),
	}

	tlsConfig.RootCAs.AppendCertsFromPEM(credSecret.Data["ca.crt"])

	prober, err = r.RedisDialer(vshnRedisServiceKey, inst.Name, inst.Namespace, org, redis.Options{
		Addr:      string(credSecret.Data["REDIS_HOST"]) + ":" + string(credSecret.Data["REDIS_PORT"]),
		Username:  string(credSecret.Data["REDIS_USERNAME"]),
		Password:  string(credSecret.Data["REDIS_PASSWORD"]),
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
		For(&vshnv1.VSHNRedis{}).
		Complete(r)
}
