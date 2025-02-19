package cmd

import (
	"context"
	"fmt"
	"github.com/vshn/appcat/v4/pkg/auth/stackgres"
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

type Maintenance interface {
	DoMaintenance(ctx context.Context) error
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
)

var maintenanceServices = map[service][]string{
	postgresql: {"postgresql"},
	redis:      {"redis"},
	minio:      {"minio"},
	mariadb:    {"mariadb"},
	keycloak:   {"keycloak"},
	nextcloud:  {"nextcloud"},
}

var serviceName service

func newMaintenanceCMD() *cobra.Command {

	command := &cobra.Command{
		Use:   "maintenance",
		Short: "Maitenance runner",
		Long:  "Run the maintenance for services",
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

	kubeClient, err := client.NewWithWatch(ctrl.GetConfigOrDie(), client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return err
	}

	var m Maintenance
	switch serviceName {
	case postgresql:

		sgNamespace := viper.GetString("SG_NAMESPACE")
		if sgNamespace == "" {
			return fmt.Errorf("missing environment variable: %s", "SG_NAMESPACE")
		}
		apiPassword := viper.GetString("API_PASSWORD")
		if apiPassword == "" {
			return fmt.Errorf("missing environment variable: %s", "API_PASSWORD")
		}

		apiUserName := viper.GetString("API_USERNAME")
		if apiUserName == "" {
			return fmt.Errorf("missing environment variable: %s", "API_USERNAME")
		}

		sClient, err := stackgres.New(apiUserName, apiPassword, sgNamespace)
		if err != nil {
			return err
		}
		m = &maintenance.PostgreSQL{
			Client:          kubeClient,
			StackgresClient: sClient,
			MaintTimeout:    time.Hour,
		}
	case redis:
		m = maintenance.NewRedis(kubeClient, getHTTPClient())

	case minio:
		m = maintenance.NewMinio(kubeClient, getHTTPClient())

	case mariadb:
		m = maintenance.NewMariaDB(kubeClient, getHTTPClient())

	case keycloak:
		m = maintenance.NewKeycloak(kubeClient, getHTTPClient())

	case nextcloud:
		m = maintenance.NewNextcloud(kubeClient, getHTTPClient())
	default:

		panic("service name is mandatory")
	}

	return m.DoMaintenance(cmd.Context())
}

func getHTTPClient() *http.Client {
	if viper.GetString("REGISTRY_USERNAME") == "" || viper.GetString("REGISTRY_PASSWORD") == "" {
		return &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return auth.GetAuthHTTPClient(viper.GetString("REGISTRY_USERNAME"), viper.GetString("REGISTRY_PASSWORD"))
}
