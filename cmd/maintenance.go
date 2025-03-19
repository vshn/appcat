package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/auth/stackgres"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/thediveo/enumflag/v2"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/auth"
	"github.com/vshn/appcat/v4/pkg/maintenance"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MaintenanceCMD specifies the cobra command for triggering the maintenance.
var (
	MaintenanceCMD = newMaintenanceCMD()
)

var (
	requiredEnvVars = []string{
		"INSTANCE_NAMESPACE",
		"CLAIM_NAME",
		"CLAIM_NAMESPACE",
		"OWNER_GROUP",
		"OWNER_KIND",
		"OWNER_VERSION",
		"SERVICE_ID",
	}
	requireAdditionalPosgresEnv = []string{
		"SG_NAMESPACE",
		"API_PASSWORD",
		"API_USERNAME",
		"VACUUM_ENABLED",
		"REPACK_ENABLED",
	}
)

type Maintenance interface {
	DoMaintenance(ctx context.Context) error
	ReleaseLatest(ctx context.Context) error
}

type service enumflag.Flag

const (
	noDefault service = iota
	postgresql
	redis
	minio
	mariadb
	keycloak
	nextcloud
	forgejo
)

var maintenanceServices = map[service][]string{
	postgresql: {"postgresql"},
	redis:      {"redis"},
	minio:      {"minio"},
	mariadb:    {"mariadb"},
	keycloak:   {"keycloak"},
	nextcloud:  {"nextcloud"},
	forgejo:    {"forgejo"},
}

var serviceName service

func newMaintenanceCMD() *cobra.Command {

	command := &cobra.Command{
		Use:   "maintenance",
		Short: "Maintenance runner",
		Long:  "Run maintenance for various services",
		RunE:  c.runMaintenance,
	}

	command.Flags().Var(
		enumflag.NewWithoutDefault(&serviceName, "service", maintenanceServices, enumflag.EnumCaseInsensitive),
		"service",
		"Specify the name of the service that should be maintained")
	err := command.MarkFlagRequired("service")
	if err != nil {
		panic(err)
	}

	return command
}

func (c *controller) runMaintenance(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	kubeClient, err := client.NewWithWatch(ctrl.GetConfigOrDie(), client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize kube client: %w", err)
	}

	if err = validateMandatoryEnvs(serviceName); err != nil {
		return fmt.Errorf("missing required environment variables: %w", err)
	}

	log := logr.FromContextOrDiscard(ctx).WithValues(
		"instanceNamespace", viper.GetString("INSTANCE_NAMESPACE"),
		"claimName", viper.GetString("CLAIM_NAME"),
		"claimNamespace", viper.GetString("CLAIM_NAMESPACE"),
		"serviceId", viper.GetString("SERVICE_ID"),
	)

	vh := getVersionHandler(kubeClient, log)

	var m Maintenance
	switch serviceName {
	case postgresql:
		if m, err = initPostgreSQL(kubeClient, vh, log); err != nil {
			return fmt.Errorf("failed to initialize postgresql: %w", err)
		}
	case redis:
		m = maintenance.NewRedis(kubeClient, getHTTPClient(), vh, log)

	case minio:
		m = maintenance.NewMinio(kubeClient, getHTTPClient(), vh, log)

	case mariadb:
		m = maintenance.NewMariaDB(kubeClient, getHTTPClient(), vh, log)

	case keycloak:
		m = maintenance.NewKeycloak(kubeClient, getHTTPClient(), vh, log)

	case nextcloud:
		m = maintenance.NewNextcloud(kubeClient, getHTTPClient(), vh, log)

	case forgejo:
		m = maintenance.NewForgejo(kubeClient, getHTTPClient(), vh, log)
	default:

		panic("service name is mandatory")
	}

	if err = errors.Join(
		m.DoMaintenance(ctx),
		m.ReleaseLatest(ctx),
	); err != nil {
		return fmt.Errorf("maintenance failed: %w", err)
	}

	return nil
}

func initPostgreSQL(kubeClient client.WithWatch, vh release.VersionHandler, log logr.Logger) (Maintenance, error) {
	sClient, err := getStackgresClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize stackgres client: %w", err)
	}
	return maintenance.NewPostgreSQL(kubeClient, sClient, vh, log), nil
}

func getHTTPClient() *http.Client {
	if viper.GetString("REGISTRY_USERNAME") == "" || viper.GetString("REGISTRY_PASSWORD") == "" {
		return &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return auth.GetAuthHTTPClient(viper.GetString("REGISTRY_USERNAME"), viper.GetString("REGISTRY_PASSWORD"))
}

func validateMandatoryEnvs(s service) error {
	for _, envVar := range requiredEnvVars {
		if !viper.IsSet(envVar) {
			return fmt.Errorf("required environment variable %s not set", envVar)
		}
	}
	if s == postgresql {
		for _, envVar := range requireAdditionalPosgresEnv {
			if !viper.IsSet(envVar) {
				return fmt.Errorf("required environment variable %s not set", envVar)
			}
		}
	}
	return nil
}

func getVersionHandler(k8sCLient client.Client, log logr.Logger) release.VersionHandler {
	return release.NewDefaultVersionHandler(
		k8sCLient,
		log,
		viper.GetString("CLAIM_NAME"),
		viper.GetString("COMPOSITE_NAME"),
		viper.GetString("CLAIM_NAMESPACE"),
		viper.GetString("OWNER_GROUP"),
		viper.GetString("OWNER_KIND"),
		viper.GetString("OWNER_VERSION"),
		viper.GetString("SERVICE_ID"),
	)
}

func getStackgresClient() (*stackgres.StackgresClient, error) {
	sClient, err := stackgres.New(viper.GetString("API_USERNAME"), viper.GetString("API_PASSWORD"), viper.GetString("SG_NAMESPACE"))
	if err != nil {
		return nil, fmt.Errorf("cannot initilize stackgres http client: %w", err)
	}
	return sClient, nil
}
