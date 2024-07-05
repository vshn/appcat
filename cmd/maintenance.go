package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/thediveo/enumflag/v2"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/maintenance"
	"github.com/vshn/appcat/v4/pkg/maintenance/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MaintenanceCMD specifies the cobra command for triggering the maintenance.
var (
	MaintenanceCMD = newMaintenanceCMD()
)

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

	switch serviceName {
	case postgresql:

		sgNamespace := viper.GetString("SG_NAMESPACE")
		if sgNamespace == "" {
			return fmt.Errorf("missing environment variable: %s", "SG_NAMESPACE")
		}

		pg := maintenance.PostgreSQL{
			Client:       kubeClient,
			SgURL:        "https://stackgres-restapi." + sgNamespace + ".svc",
			MaintTimeout: time.Hour,
		}
		return pg.DoMaintenance(cmd.Context())
	case redis:
		r := maintenance.NewRedis(kubeClient, getHTTPClient())
		return r.DoMaintenance(cmd.Context())

	case minio:
		m := maintenance.NewMinio(kubeClient, getHTTPClient())
		return m.DoMaintenance(cmd.Context())

	case mariadb:
		m := maintenance.NewMariaDB(kubeClient, getHTTPClient())
		return m.DoMaintenance(cmd.Context())

	case keycloak:
		k := maintenance.NewKeycloak(kubeClient, getHTTPClient())
		return k.DoMaintenance(cmd.Context())

	case nextcloud:
		k := maintenance.NewNextcloud(kubeClient, getHTTPClient())
		return k.DoMaintenance(cmd.Context())
	}

	return nil
}

func getHTTPClient() *http.Client {
	if viper.GetString("REGISTRY_USERNAME") == "" || viper.GetString("REGISTRY_PASSWORD") == "" {
		return &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return auth.GetAuthHTTPClient(viper.GetString("REGISTRY_USERNAME"), viper.GetString("REGISTRY_PASSWORD"))
}
