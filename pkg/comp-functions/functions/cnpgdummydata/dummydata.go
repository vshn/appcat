// Package cnpgdummydata is a PoC composition function step that emits a
// Kubernetes Job inserting dummy data into a CNPG-managed PostgreSQL cluster
// once that cluster reports healthy.
//
// The cluster is observed via Crossplane's ExtraResources mechanism (decoupled
// from provider-kubernetes' reconcile loop) — Crossplane fetches the Cluster
// fresh from the API server on each function invocation.
package cnpgdummydata

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// extraResourceKey identifies the CNPG cluster ExtraResource in req/resp.
	extraResourceKey = "cnpg-cluster"
	// clusterName is hardcoded for CNPG-backed VSHNPostgreSQL — always "postgresql"
	// in the instance namespace.
	clusterName = "postgresql"
	// healthyPhase is the CNPG Cluster status.phase value indicating readiness.
	healthyPhase = "Cluster in healthy state"
	// jobNamePrefix produces stable Job names per instance — re-emission is a no-op.
	jobNamePrefix = "dummy-data-"
	// jobSAName runs the operation Job; granted just enough to read the
	// CNPG-managed connection secret in the instance namespace.
	jobSAName = "dummy-data-runner"
)

// EmitDummyDataJob is the composition function step. On each reconcile it:
//  1. Declares a requirement for the CNPG Cluster as an ExtraResource.
//  2. If Crossplane has already fetched it and the cluster reports healthy,
//     emits a Job (stable name) running `appcat operation insert-dummy`.
//  3. Otherwise no-ops, awaiting the next reconcile.
func EmitDummyDataJob(_ context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	instanceNS := comp.GetInstanceNamespace()
	if instanceNS == "" {
		return runtime.NewNormalResult("instance namespace not yet populated; deferring")
	}

	// MatchName w/o Namespace returns nothing for namespaced kinds in this
	// Crossplane version (SDK v0.4 ResourceSelector has no Namespace field;
	// added in v0.5+). MatchLabels does a label-selector list — that works
	// cluster-wide. Filter by namespace inside the function below.
	svc.RequireExtraResource(extraResourceKey, &fnv1.ResourceSelector{
		ApiVersion: "postgresql.cnpg.io/v1",
		Kind:       "Cluster",
		Match: &fnv1.ResourceSelector_MatchLabels{
			MatchLabels: &fnv1.MatchLabels{Labels: map[string]string{
				// Helm sets app.kubernetes.io/instance to the composite name
				// (release name = composite). Unique per instance — no namespace
				// post-filter needed, but we keep it as a defense in depth.
				"app.kubernetes.io/instance": comp.GetName(),
			}},
		},
	})

	extras, err := svc.GetExtraResources(extraResourceKey)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot read extra resources: %w", err))
	}
	seenNS := make([]string, 0, len(extras))
	for _, e := range extras {
		if e.Resource != nil {
			seenNS = append(seenNS, e.Resource.GetNamespace()+"/"+e.Resource.GetName())
		}
	}
	svc.Log.Info("cnpgdummydata extras debug", "count", len(extras), "wantNS", instanceNS, "got", seenNS)

	cluster := pickClusterInNamespace(extras, instanceNS)
	if cluster == nil {
		return runtime.NewNormalResult(fmt.Sprintf("awaiting CNPG Cluster fetch from extra resources (count=%d wantNS=%s got=%v)", len(extras), instanceNS, seenNS))
	}
	phase, found, err := unstructuredString(cluster, "status", "phase")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot read cluster status.phase: %v", err))
	}
	if !found || phase != healthyPhase {
		return runtime.NewNormalResult(fmt.Sprintf("CNPG cluster not yet healthy (phase=%q); deferring", phase))
	}

	if err := emitJobRBAC(svc, comp.GetName(), instanceNS); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot emit dummy-data RBAC: %w", err))
	}

	jobName := jobNamePrefix + comp.GetName()
	op := common.OperationJob{
		Name:           jobName,
		Namespace:      instanceNS,
		Subcommand:     "insert-dummy",
		Args:           []string{"--instance=" + clusterName, "--namespace=" + instanceNS, "--composite=" + comp.GetName()},
		ServiceAccount: jobSAName,
		Env: []corev1.EnvVar{
			{Name: "INSTANCE_NAMESPACE", Value: instanceNS},
			{Name: "COMPOSITE_NAME", Value: comp.GetName()},
		},
	}
	if err := common.EmitOperationJob(svc, op); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot emit dummy-data job: %w", err))
	}
	return runtime.NewNormalResult("emitted dummy-data job " + jobName)
}

// emitJobRBAC emits a ServiceAccount + Role + RoleBinding granting the
// operation Job permission to read the CNPG-managed connection secret
// (`<composite>-connection`) in the instance namespace.
func emitJobRBAC(svc *runtime.ServiceRuntime, composite, instanceNS string) error {
	connectionSecret := composite + "-connection"

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: jobSAName, Namespace: instanceNS},
	}
	if err := svc.SetDesiredKubeObject(sa, jobNamePrefix+composite+"-sa", runtime.KubeOptionAllowDeletion); err != nil {
		return err
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: jobSAName, Namespace: instanceNS},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: []string{connectionSecret},
			Verbs:         []string{"get"},
		}},
	}
	if err := svc.SetDesiredKubeObject(role, jobNamePrefix+composite+"-role", runtime.KubeOptionAllowDeletion); err != nil {
		return err
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: jobSAName, Namespace: instanceNS},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: jobSAName},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: jobSAName, Namespace: instanceNS}},
	}
	return svc.SetDesiredKubeObject(rb, jobNamePrefix+composite+"-rb", runtime.KubeOptionAllowDeletion)
}

// pickClusterInNamespace returns the unstructured map of the first ExtraResource
// whose metadata.namespace equals ns, or nil if none. Needed because v0.4 of the
// SDK can't filter by namespace at selector time.
func pickClusterInNamespace(extras []resource.Extra, ns string) map[string]interface{} {
	for _, e := range extras {
		if e.Resource == nil {
			continue
		}
		if e.Resource.GetNamespace() == ns {
			return e.Resource.Object
		}
	}
	return nil
}

// unstructuredString reads a string at a nested path. Wraps the apimachinery
// helper to keep the call site short.
func unstructuredString(obj map[string]interface{}, path ...string) (string, bool, error) {
	cur := interface{}(obj)
	for _, p := range path {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return "", false, nil
		}
		cur, ok = m[p]
		if !ok {
			return "", false, nil
		}
	}
	s, ok := cur.(string)
	if !ok {
		return "", false, fmt.Errorf("value at %v is not a string", path)
	}
	return s, true, nil
}
