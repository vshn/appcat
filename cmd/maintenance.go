package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/thediveo/enumflag/v2"
	"github.com/vshn/appcat/pkg"
	"github.com/vshn/appcat/pkg/maintenance"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MaintenanceCMD specifies the cobra command for triggering the maintenance.
var (
	MaintenanceCMD    = newMaintenanceCMD()
	instanceNamespace string
)

type service enumflag.Flag

const (
	noDefault service = iota
	postgresql
)

var maintenanceServices = map[service][]string{
	postgresql: {"postgresql"},
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

	kubeClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{
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
			Client: kubeClient,
			SgURL:  "https://stackgres-restapi." + sgNamespace + ".svc",
		}
		return pg.DoMaintenance(cmd.Context())
	}

	return nil
}
