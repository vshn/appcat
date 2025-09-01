package vshnpostgres

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const pgBouncerSettingName = "pgbouncer-settings"

func addPGBouncerSettings(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}
	if comp.Spec.Parameters.UseCNPG {
		svc.Log.Info("Skipping addPGBouncerSettings because we're using CNPG")
		return nil
	}

	comp, err = getVSHNPostgreSQL(ctx, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	if comp.Spec.Parameters.Service.PgBouncerSettings == nil {
		return nil
	}

	bouncerSettings := &sgv1.SGPoolingConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgBouncerSettingName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: sgv1.SGPoolingConfigSpec{
			PgBouncer: &sgv1.SGPoolingConfigSpecPgBouncer{
				PgbouncerIni: comp.Spec.Parameters.Service.PgBouncerSettings,
			},
		},
	}

	err = svc.SetDesiredKubeObject(bouncerSettings, comp.GetName()+"-pooling-config")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot apply pooling config: %s", err))
	}

	cluster := &sgv1.SGCluster{}
	err = svc.GetDesiredKubeObject(cluster, "cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get cluster: %s", err))
	}

	cluster.Spec.Configurations.SgPoolingConfig = ptr.To(pgBouncerSettingName)

	err = svc.SetDesiredKubeObjectWithName(cluster, comp.GetName()+"-cluster", "cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot set cluster: %s", err))
	}

	return nil
}
