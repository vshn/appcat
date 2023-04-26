package command

import (
	"errors"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	appcatv1 "github.com/vshn/appcat-apiserver/apis/appcat/v1"
	"github.com/vshn/appcat-apiserver/apiserver/appcat"
	"github.com/vshn/appcat-apiserver/apiserver/vshn/postgres"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"
	ctrl "sigs.k8s.io/controller-runtime"
)

type apiServerCommand struct {
	appcatEnabled      bool
	vshnBackupsEnabled bool
}

func Command() *cli.Command {
	cmd := &apiServerCommand{}
	return &cli.Command{
		Name:   "api-server",
		Usage:  "Start api-server",
		Action: cmd.execute,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "appcat-handler",
				Usage:       "Enables AppCat handler for this Api Server",
				Value:       true,
				EnvVars:     []string{"APPCAT_HANDLER_ENABLED"},
				Destination: &cmd.appcatEnabled,
			},
			&cli.BoolFlag{
				Name:        "postgres-backup-handler",
				Usage:       "Enables PostgreSQL backup handler for this Api Server",
				Value:       false,
				EnvVars:     []string{"VSHN_POSTGRES_BACKUP_HANDLER_ENABLED"},
				Destination: &cmd.vshnBackupsEnabled,
			},
		},
	}
}

func (a *apiServerCommand) execute(ctx *cli.Context) error {
	log := logr.FromContextOrDiscard(ctx.Context)

	ctrl.SetLogger(log)

	if !a.appcatEnabled && !a.vshnBackupsEnabled {
		err := errors.New("cannot start api server")
		log.Error(err, "Enable at least one handler for the API Server")
		return err
	}

	b := builder.APIServer

	if a.appcatEnabled {
		b.WithResourceAndHandler(&appcatv1.AppCat{}, appcat.New())
	}

	if a.vshnBackupsEnabled {
		b.WithResourceAndHandler(&appcatv1.VSHNPostgresBackup{}, postgres.New())
	}

	b.WithoutEtcd().
		ExposeLoopbackAuthorizer().
		ExposeLoopbackMasterClientConfig()

	cmd, err := b.Build()
	if err != nil {
		log.Error(err, "Unable to load build API Server")
		return err
	}

	if err = cmd.Execute(); err != nil {
		log.Error(err, "Unable to finish execution of API Server")
		return err
	}

	return nil
}
