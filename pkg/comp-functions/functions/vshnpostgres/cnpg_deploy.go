package vshnpostgres

import (
	"context"
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
		return runtime.NewFatalResult(fmt.Errorf("deployPostgresSQLUsingCNPG() called but useCnpg is false"))
	}

	// Deploy
	values, err := createCnpgHelmValues(ctx, svc, comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create helm values: %w", err))
	}

	svc.Log.Info("Creating Helm release for CNPG PostgreSQL")
	overrides := common.HelmReleaseOverrides{
		Repository: svc.Config.Data["cnpgClusterChartSource"],
		Version:    svc.Config.Data["cnpgClusterChartVersion"],
		Chart:      svc.Config.Data["cnpgClusterChartName"],
	}

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

// Generate CNPG cluster helm chart values
func createCnpgHelmValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) (map[string]any, error) {
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
	svc.Log.Info("Setting postgresSettings")
	pgConf, err := getPgSettingsMap(comp.Spec.Parameters.Service.PostgreSQLSettings)
	if err != nil {
		return map[string]any{}, fmt.Errorf("cannot get pg settings: %w", err)
	}

	for k, v := range pgConf {
		err = common.SetNestedObjectValue(values, []string{"cluster", "postgresql", "parameters", k}, v)
		if err != nil {
			return map[string]any{}, fmt.Errorf("cannot set pg settings %s=%s: %w", k, v, err)
		}
	}

	// Compute resources
	svc.Log.Info("Fetching and setting compute resources")
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])
	res, err := getResourcesForPlan(ctx, svc, comp, plan)
	if err != nil {
		return map[string]any{}, fmt.Errorf("could not set resources: %w", err)
	}

	err = setResourcesCnpg(values, res)
	if err != nil {
		return map[string]any{}, fmt.Errorf("cannot set resources: %w", err)
	}

	return values, nil
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

// Get resources for a given plan
func getResourcesForPlan(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, plan string) (common.Resources, error) {
	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		return common.Resources{}, err
	}

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) > 0 {
		return common.Resources{}, errors.Join(errs...)
	}

	return res, nil
}
