package vshnpostgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// Deploy PostgresQL using the CNPG cluster helm chart
func deployPostgresSQLUsingCNPG(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if !comp.Spec.Parameters.UseCNPG {
		return nil
	}

	// https://github.com/cloudnative-pg/charts/blob/main/charts/cluster/values.yaml
	values := map[string]any{
		"version": map[string]string{
			"postgres": comp.Spec.Parameters.Service.MajorVersion,
		},
		"cluster": map[string]any{
			"instances": 1, // For the moment we only support single instance deployments
			"postgresql": map[string]any{
				"parameters": map[string]any{},
			},
			"certificates": map[string]string{
				"serverCASecret":  certificateSecretName,
				"serverTLSSecret": certificateSecretName,
			},
			// The following will be overwritten by setResources() later
			"storage": map[string]any{},
			"resources": map[string]any{
				"requests": map[string]any{},
				"limits":   map[string]any{},
			},
		},
	}

	// PostgreSQLSettings
	svc.Log.Info("Fetching postgresSettings")
	pgConf, err := getPgSettingsMap(comp.Spec.Parameters.Service.PostgreSQLSettings)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get pg settings: %w", err))
	}

	svc.Log.Info("Setting postgresSettings")
	for k, v := range pgConf {
		err = common.SetNestedObjectValue(values, []string{"cluster", "postgresql", "parameters", k}, v)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot set pg settings %s=%s: %w", k, v, err))
		}
	}

	// Compute resources
	svc.Log.Info("Fetching and setting compute resources")
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("could not fetch plans from config: %w", err))
	}
	svc.Log.Info("Got resources from plan", "plan", plan, "resources", resources)

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) > 0 {
		svc.Log.Error(fmt.Errorf("could not get resources"), "errors", errors.Join(errs...))
	}
	svc.Log.Info("Final resources to use", "resources", res)

	err = setResourcesCnpg(values, res)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set resources: %w", err))
	}

	o, _ := json.Marshal(values)
	svc.Log.Info("Deploying PostgreSQL using CNPG", "values", string(o))

	overrides := common.HelmReleaseOverrides{
		Repository: "https://cloudnative-pg.github.io/charts",
		Version:    "0.3.1",
		Chart:      "cluster",
	}

	svc.Log.Info("Creating Helm release for CNPG PostgreSQL")
	release, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-cnpg", overrides)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create release: %w", err))
	}

	err = svc.SetDesiredComposedResource(release)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set desired release: %w", err))
	}
	return nil
}

// Set compute resources in the values map
func setResourcesCnpg(values map[string]any, resources common.Resources) error {
	err := common.SetNestedObjectValue(values, []string{"cluster", "resources", "limits", "cpu"}, resources.CPU.String())
	if err != nil {
		return fmt.Errorf("cannot set cpu limits: %w", err)
	}
	err = common.SetNestedObjectValue(values, []string{"cluster", "resources", "requests", "cpu"}, resources.ReqCPU.String())
	if err != nil {
		return fmt.Errorf("cannot set cpu requests: %w", err)
	}

	err = common.SetNestedObjectValue(values, []string{"cluster", "resources", "limits", "memory"}, resources.Mem.String())
	if err != nil {
		return fmt.Errorf("cannot set memory limits: %w", err)
	}
	err = common.SetNestedObjectValue(values, []string{"cluster", "resources", "requests", "memory"}, resources.ReqMem.String())
	if err != nil {
		return fmt.Errorf("cannot set memory requests: %w", err)
	}

	err = common.SetNestedObjectValue(values, []string{"cluster", "storage", "size"}, resources.Disk.String())
	if err != nil {
		return fmt.Errorf("cannot set disk size: %w", err)
	}

	return nil
}
