/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sli_exporter

import (
	"context"
	"fmt"
	"github.com/vshn/appcat-apiserver/pkg/sli-exporter/probes"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	vshnv1 "github.com/vshn/appcat-apiserver/apis/vshn/v1"
)

var vshnpostgresqlsServiceKey = "VSHNPostgreSQL"

// VSHNPostgreSQLReconciler reconciles a VSHNPostgreSQL object
type VSHNPostgreSQLReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ProbeManager       probeManager
	StartupGracePeriod time.Duration
	PostgreDialer      func(service, name, namespace, dsn string, ops ...func(*pgxpool.Config) error) (*probes.PostgreSQL, error)
}

type probeManager interface {
	StartProbe(p probes.Prober)
	StopProbe(p probes.ProbeInfo)
}

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnpostgresqls,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnpostgresqls/status,verbs=get

// Reconcile start or stops a prober for a VSHNPostgreSQL instance.
// Will only probe an instance once it is ready or after the StartupGracePeriod.
func (r *VSHNPostgreSQLReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("namespace", req.Namespace, "instance", req.Name)
	res := ctrl.Result{}

	inst := &vshnv1.VSHNPostgreSQL{}
	err := r.Get(ctx, req.NamespacedName, inst)

	if apierrors.IsNotFound(err) || inst.DeletionTimestamp != nil {
		l.Info("Stopping Probe")
		r.ProbeManager.StopProbe(probes.ProbeInfo{
			Service:   vshnpostgresqlsServiceKey,
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

	probe, err := r.fetchProberFor(ctx, inst)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if apierrors.IsNotFound(err) {
		l.WithValues("credentials", inst.Spec.WriteConnectionSecretToRef.Name, "error", err.Error()).
			Info("Failed to find credentials. Backing off")
		res.Requeue = true
		res.RequeueAfter = 30 * time.Second

		if time.Now().Sub(inst.GetCreationTimestamp().Time) < r.StartupGracePeriod {
			// Instance is starting up. Postpone probing until ready.
			return res, nil
		}

		// Create a pobe that will always fail
		probe, err = probes.NewFailingPostgreSQL(vshnpostgresqlsServiceKey, inst.Name, inst.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	l.Info("Starting Probe")
	r.ProbeManager.StartProbe(probe)
	return res, nil
}

func (r VSHNPostgreSQLReconciler) fetchProberFor(ctx context.Context, inst *vshnv1.VSHNPostgreSQL) (probes.Prober, error) {

	credSecret := corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      inst.Spec.WriteConnectionSecretToRef.Name,
		Namespace: inst.Namespace,
	}, &credSecret)

	if err != nil {
		return nil, err
	}

	probe, err := r.PostgreDialer(vshnpostgresqlsServiceKey, inst.Name, inst.Namespace,
		fmt.Sprintf(
			"postgresql://%s:%s@%s:%s/%s?sslmode=verify-ca",
			credSecret.Data["POSTGRESQL_USER"],
			credSecret.Data["POSTGRESQL_PASSWORD"],
			credSecret.Data["POSTGRESQL_HOST"],
			credSecret.Data["POSTGRESQL_PORT"],
			credSecret.Data["POSTGRESQL_DB"],
		), probes.PGWithCA(credSecret.Data["ca.crt"]))
	if err != nil {
		return nil, err
	}
	return probe, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VSHNPostgreSQLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.VSHNPostgreSQL{}).
		Complete(r)
}
