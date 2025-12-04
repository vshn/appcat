package slireconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ClaimNamespaceLabel = "crossplane.io/claim-namespace"
	ClaimNameLabel      = "crossplane.io/claim-name"
)

var (
	errNotReady = fmt.Errorf("Resource is not yet ready")
)

// Reconciler is a generic reconciler logs that acts on xrds.
type Reconciler struct {
	inst               Service
	l                  logr.Logger
	pm                 ProbeManager
	serviceKey         string
	nn                 types.NamespacedName
	startupGracePeriod time.Duration
	fetchProberFor     func(context.Context, Service) (probes.Prober, error)
	client             client.Client
	scClient           client.Client
}

// New returns a new Reconciler
func New(inst Service, l logr.Logger, pm ProbeManager, serviceKey string, nn types.NamespacedName,
	client client.Client, startupGracePeriod time.Duration,
	fetchProberFor func(context.Context, Service) (probes.Prober, error),
	scClient client.Client) *Reconciler {
	return &Reconciler{
		inst:               inst,
		l:                  l,
		pm:                 pm,
		serviceKey:         serviceKey,
		nn:                 nn,
		client:             client,
		startupGracePeriod: startupGracePeriod,
		fetchProberFor:     fetchProberFor,
		scClient:           scClient,
	}
}

// Reconcile contains the actual reconcilation logic to start the probers.
// It is designed to act on the xrd, NOT the claims.
func (r *Reconciler) Reconcile(ctx context.Context) (ctrl.Result, error) {

	res := ctrl.Result{}

	err := r.client.Get(ctx, r.nn, r.inst)

	if apierrors.IsNotFound(err) || r.inst.GetDeletionTimestamp() != nil {
		r.l.Info("Stopping Probe")
		// r.pm.StopProbe(probes.NewProbeInfo(r.serviceKey, r.nn, r.inst))
		r.pm.StopProbe(probes.ProbeInfo{
			Service: r.serviceKey,
			Name:    r.nn.Name,
		})
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if r.inst.GetWriteConnectionSecretToReference() == nil || r.inst.GetWriteConnectionSecretToReference().Name == "" {
		r.l.Info("No connection secret requested. Skipping.")
		return ctrl.Result{}, nil
	}

	// Check if instance is suspended (instances = 0)
	// When suspended, we stop probing to prevent false monitoring alerts
	if instancesGetter, ok := r.inst.(common.InstancesGetter); ok {
		if instancesGetter.GetInstances() == 0 {
			r.l.Info("Instance is suspended, stopping probe")
			r.pm.StopProbe(probes.ProbeInfo{
				Service: r.serviceKey,
				Name:    r.nn.Name,
			})
			return ctrl.Result{}, nil
		}
	}

	if time.Since(r.inst.GetCreationTimestamp().Time) < r.startupGracePeriod {
		retry := r.startupGracePeriod - time.Since(r.inst.GetCreationTimestamp().Time)
		r.l.Info(fmt.Sprintf("Instance is starting up. Postpone probing until ready, retry in %s", retry.String()))
		res.Requeue = true
		res.RequeueAfter = retry
		return res, nil
	}

	namespaceExists, err := r.namespaceExists(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !namespaceExists {
		return ctrl.Result{}, nil
	}

	claimNamespace := r.inst.GetLabels()[ClaimNamespaceLabel]
	instanceNamespace := r.inst.(common.InstanceNamespaceGetter).GetInstanceNamespace()

	probe, err := r.fetchProberFor(ctx, r.inst)
	// By using the composite the credential secret is available instantly, but initially empty.
	if err != nil && (apierrors.IsNotFound(err) || err == errNotReady) {
		r.l.WithValues("credentials", r.inst.GetWriteConnectionSecretToReference().Name, "error", err.Error()).
			Info("Failed to find credentials. Backing off")
		res.Requeue = true
		res.RequeueAfter = 30 * time.Second

		probe, err = probes.NewFailingProbe(r.serviceKey, r.inst.GetName(), claimNamespace, instanceNamespace, err)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	r.l.Info("Starting Probe")
	r.pm.StartProbe(probe)
	return res, nil
}

// namespaceExists detects wether or not the namespace exists in the service
// cluster. If it doesn't exist then we skip the reconcilation for this instance.
func (r Reconciler) namespaceExists(ctx context.Context) (bool, error) {
	comp, ok := r.inst.(common.InstanceNamespaceGetter)
	if !ok {
		return false, fmt.Errorf("resource does not implement common.InstanceNamespaceGetter")
	}

	ns := &corev1.Namespace{}

	err := r.scClient.Get(ctx, types.NamespacedName{Name: comp.GetInstanceNamespace()}, ns)
	if err != nil {
		if errors.IsNotFound(err) {
			r.l.Info("instance doesn't have a instance namespace on this cluster, skipping")
			return false, nil
		}
		return false, err
	}

	return true, nil
}
