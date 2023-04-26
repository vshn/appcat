package command

import (
	"github.com/urfave/cli/v2"
	appcatv1 "github.com/vshn/appcat-apiserver/apis/appcat/v1"
	"github.com/vshn/appcat-apiserver/apiserver/appcat"
	"github.com/vshn/appcat-apiserver/apiserver/vshn/postgres"
	"k8s.io/klog/v2"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:            "api-server",
		Usage:           "Start api-server",
		Action:          execute,
		SkipFlagParsing: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "appcat-handler",
				Usage:   "Enables AppCat handler for this Api Server",
				Value:   true,
				Aliases: []string{"a"},
				EnvVars: []string{"APPCAT_HANDLER_ENABLED"},
			},
			&cli.BoolFlag{
				Name:    "postgres-backup-handler",
				Usage:   "Enables PostgreSQL backup handler for this Api Server",
				Value:   false,
				Aliases: []string{"pb"},
				EnvVars: []string{"VSHN_POSTGRES_BACKUP_HANDLER_ENABLED"},
			},
		},
	}
}

func execute(ctx *cli.Context) error {
	appcatEnabled := ctx.Bool("appcat-handler")
	vshnBackupsEnabled := ctx.Bool("postgres-backup-handler")

	if !appcatEnabled && !vshnBackupsEnabled {
		klog.Fatal("Handlers are not enabled, please set at least one of APPCAT_HANDLER_ENABLED | VSHN_POSTGRES_BACKUP_HANDLER_ENABLED env variables to true")
	}

	b := builder.APIServer

	if appcatEnabled {
		b.WithResourceAndHandler(&appcatv1.AppCat{}, appcat.New())
	}

	if vshnBackupsEnabled {
		b.WithResourceAndHandler(&appcatv1.VSHNPostgresBackup{}, postgres.New())
	}

	b.WithoutEtcd().
		ExposeLoopbackAuthorizer().
		ExposeLoopbackMasterClientConfig()

	cmd, err := b.Build()
	if err != nil {
		klog.Fatal(err)
	}

	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}

	return nil
}
