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

package vshnpostgresqlcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	slireconciler "github.com/vshn/appcat/v4/pkg/sliexporter/sli_reconciler"

	"github.com/jackc/pgx/v5/pgxpool"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

var (
	vshnpostgresqlsServiceKey = "VSHNPostgreSQL"
	errNotReady               = fmt.Errorf("Resource is not yet ready")
)

// VSHNPostgreSQLReconciler reconciles a VSHNPostgreSQL object
type VSHNPostgreSQLReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ProbeManager       probeManager
	StartupGracePeriod time.Duration
	PostgreDialer      func(service, name, namespace, dsn, organization, serviceLevel string, ha bool, ops ...func(*pgxpool.Config) error) (*probes.PostgreSQL, error)
}

type probeManager interface {
	slireconciler.ProbeManager
}

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls/status,verbs=get
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnpostgresqls,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshnpostgresqls/status,verbs=get

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile start or stops a prober for a VSHNPostgreSQL instance.
// Will only probe an instance once it is ready or after the StartupGracePeriod.
func (r *VSHNPostgreSQLReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("namespace", req.Namespace, "instance", req.Name)

	inst := &vshnv1.XVSHNPostgreSQL{}

	reconciler := slireconciler.New(inst, l, r.ProbeManager, vshnpostgresqlsServiceKey, req.NamespacedName, r.Client, r.StartupGracePeriod, r.fetchProberFor)

	return reconciler.Reconcile(ctx)
}

func (r VSHNPostgreSQLReconciler) fetchProberFor(ctx context.Context, obj slireconciler.Service) (probes.Prober, error) {
	inst, ok := obj.(*vshnv1.XVSHNPostgreSQL)
	if !ok {
		return nil, fmt.Errorf("cannot start probe, object not a valid VSHNPostgreSQL")
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

	ns := &corev1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: inst.GetLabels()[slireconciler.ClaimNamespaceLabel]}, ns)
	if err != nil {
		return nil, err
	}

	org := ns.GetLabels()[utils.OrgLabelName]
	sla := inst.Spec.Parameters.Service.ServiceLevel
	if sla == "" {
		sla = vshnv1.BestEffort
	}

	ha := true
	if inst.Spec.Parameters.Instances == 1 {
		ha = false
	}

	sslmode := "verify-ca"
	if !inst.Spec.Parameters.Service.TLS.Enabled {
		sslmode = "disable"
	}

	probe, err := r.PostgreDialer(vshnpostgresqlsServiceKey, inst.GetName(), inst.GetLabels()[slireconciler.ClaimNamespaceLabel],
		fmt.Sprintf(
			"postgresql://%s:%s@%s:%s/%s?sslmode=%s",
			credSecret.Data["POSTGRESQL_USER"],
			credSecret.Data["POSTGRESQL_PASSWORD"],
			credSecret.Data["POSTGRESQL_HOST"],
			credSecret.Data["POSTGRESQL_PORT"],
			credSecret.Data["POSTGRESQL_DB"],
			sslmode,
		), org, string(sla), ha,
		probes.PGWithCA(credSecret.Data["ca.crt"], inst.Spec.Parameters.Service.TLS.Enabled),
	)
	if err != nil {
		return nil, err
	}

	return probe, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VSHNPostgreSQLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.XVSHNPostgreSQL{}).
		Complete(r)
}

func (r *VSHNPostgreSQLReconciler) areCredentialsAvailable(secret *corev1.Secret) bool {

	_, ok := secret.Data["POSTGRESQL_USER"]
	if !ok {
		return false
	}
	_, ok = secret.Data["POSTGRESQL_PASSWORD"]
	if !ok {
		return false
	}
	_, ok = secret.Data["POSTGRESQL_HOST"]
	if !ok {
		return false
	}
	_, ok = secret.Data["POSTGRESQL_PORT"]
	if !ok {
		return false
	}
	_, ok = secret.Data["POSTGRESQL_DB"]
	if !ok {
		return false
	}
	_, ok = secret.Data["ca.crt"]
	if !ok {
		return false
	}

	return ok
}
