package vshnpostgres

import (
	"context"
	"fmt"
	"sort"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	timescaleExtName   = "timescaledb"
	configResourceName = "pg-conf"
	sharedLibraries    = "shared_preload_libraries"
)

// AddExtensions adds the user specified extensions to the SGCluster.
func AddExtensions(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Starting extensions function")

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	extensionsMap := map[string]stackgresv1.SGClusterSpecPostgresExtensionsItem{}

	for _, ext := range comp.Spec.Parameters.Service.Extensions {
		extensionsMap[ext.Name] = stackgresv1.SGClusterSpecPostgresExtensionsItem{
			Name: ext.Name,
		}
		log.Info("enable extension", "name", ext.Name, "cluster", comp.GetName(), "namespace", comp.GetNamespace())
	}

	// ensure pg_repack is always enabled
	extensionsMap["pg_repack"] = stackgresv1.SGClusterSpecPostgresExtensionsItem{
		Name:    "pg_repack",
		Version: ptr.To("1.5.0"),
	}

	if _, ok := extensionsMap["timescaledb"]; ok {
		err := enableTimescaleDB(ctx, svc)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot add timescaldb to config: %w", err))
		}
	} else {
		err := disableTimescaleDB(ctx, svc)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot ensure timescaldb absent from config: %w", err))
		}
	}

	cluster := &stackgresv1.SGCluster{}
	err = svc.GetDesiredKubeObject(cluster, "cluster")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("not able to get cluster: %w", err))
	}

	finalExtensions := []stackgresv1.SGClusterSpecPostgresExtensionsItem{}

	for _, ext := range extensionsMap {
		finalExtensions = append(finalExtensions, ext)
	}

	sort.Slice(finalExtensions, func(i, j int) bool {
		return finalExtensions[i].Name < finalExtensions[j].Name
	})

	cluster.Spec.Postgres.Extensions = &finalExtensions

	err = svc.SetDesiredKubeObjectWithName(cluster, comp.GetName()+"-cluster", "cluster")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot save cluster to functionIO: %w", err))
	}

	return nil
}

func enableTimescaleDB(ctx context.Context, svc *runtime.ServiceRuntime) error {

	config := &stackgresv1.SGPostgresConfig{}

	err := svc.GetDesiredKubeObject(config, configResourceName)
	if err != nil && err == runtime.ErrNotFound {
		controllerruntime.LoggerFrom(ctx).Info("no pg-conf found")
		return nil
	} else if err != nil {
		return err
	}

	if _, ok := config.Spec.PostgresqlConf[sharedLibraries]; !ok {
		config.Spec.PostgresqlConf[sharedLibraries] = timescaleExtName
	}

	if !strings.Contains(config.Spec.PostgresqlConf[sharedLibraries], timescaleExtName) {
		toAppend := config.Spec.PostgresqlConf[sharedLibraries]
		config.Spec.PostgresqlConf[sharedLibraries] = fmt.Sprintf("%s,%s", toAppend, timescaleExtName)
	}

	return svc.SetDesiredKubeObject(config, configResourceName)
}

func disableTimescaleDB(ctx context.Context, svc *runtime.ServiceRuntime) error {

	config := &stackgresv1.SGPostgresConfig{}

	err := svc.GetDesiredKubeObject(config, configResourceName)
	if err != nil && err == runtime.ErrNotFound {
		controllerruntime.LoggerFrom(ctx).Info("no pg-conf found")
		return nil
	} else if err != nil {
		return err
	}

	if _, ok := config.Spec.PostgresqlConf[sharedLibraries]; !ok {
		return nil
	}

	elements := strings.Split(config.Spec.PostgresqlConf[sharedLibraries], ",")
	finalElements := []string{}

	for _, elem := range elements {
		if !strings.Contains(elem, timescaleExtName) {
			finalElements = append(finalElements, strings.TrimSpace(elem))
		}
	}

	config.Spec.PostgresqlConf[sharedLibraries] = strings.Join(finalElements, ", ")

	return svc.SetDesiredKubeObject(config, configResourceName)
}
