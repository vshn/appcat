package slireconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
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
}

// New returns a new Reconciler
func New(inst Service, l logr.Logger, pm ProbeManager, serviceKey string, nn types.NamespacedName,
	client client.Client, startupGracePeriod time.Duration, fetchProberFor func(context.Context, Service) (probes.Prober, error)) *Reconciler {
	return &Reconciler{
		inst:               inst,
		l:                  l,
		pm:                 pm,
		serviceKey:         serviceKey,
		nn:                 nn,
		client:             client,
		startupGracePeriod: startupGracePeriod,
		fetchProberFor:     fetchProberFor,
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
			Service:   r.serviceKey,
			Name:      r.nn.Name,
			Namespace: r.nn.Namespace,
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

	if time.Since(r.inst.GetCreationTimestamp().Time) < r.startupGracePeriod {
		retry := r.startupGracePeriod - time.Since(r.inst.GetCreationTimestamp().Time)
		r.l.Info(fmt.Sprintf("Instance is starting up. Postpone probing until ready, retry in %s", retry.String()))
		res.Requeue = true
		res.RequeueAfter = retry
		return res, nil
	}

	probe, err := r.fetchProberFor(ctx, r.inst)
	// By using the composite the credential secret is available instantly, but initially empty.
	if err != nil && (apierrors.IsNotFound(err) || err == errNotReady) {
		r.l.WithValues("credentials", r.inst.GetWriteConnectionSecretToReference().Name, "error", err.Error()).
			Info("Failed to find credentials. Backing off")
		res.Requeue = true
		res.RequeueAfter = 30 * time.Second

		// Create a pobe that will always fail
		probe, err = probes.NewFailingProbe(r.serviceKey, r.inst.GetName(), r.inst.GetLabels()[ClaimNamespaceLabel], err)
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
