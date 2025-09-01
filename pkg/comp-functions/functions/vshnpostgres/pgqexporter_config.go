package vshnpostgres

import (
	"context"
	_ "embed"
	"fmt"

	v1 "github.com/crossplane/function-sdk-go/proto/v1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
	This code ensures we have minimalistic postgresql-exporter configuration
	It prevents high memory usage for big databases
*/
//go:embed scripts/queries.yml
var queries string

func PgExporterConfig(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *v1.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if comp.Spec.Parameters.UseCNPG {
		svc.Log.Info("Skipping PgExporterConfig because we're using CNPG")
		return nil
	}

	// get configmap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"stackgres.io/reconciliation-pause": "true",
			},
			Name:      comp.GetName() + "-prometheus-postgres-exporter-config",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string]string{
			"queries.yaml": queries,
		},
	}
	xRef := xkube.Reference{
		DependsOn: &xkube.DependsOn{
			Name: comp.GetName() + "-cluster",
		},
	}
	// add crossplane object containing ConfigMap
	err = svc.SetDesiredKubeObjectWithName(configMap, comp.GetName()+"-prometheus-postgres-exporter-config", comp.GetName(), runtime.KubeOptionAddRefs(xRef))
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add ConfigMap: %s", err))
	}
	return nil
}
