package cmd

import (
	"github.com/spf13/cobra"
	appcatv1 "github.com/vshn/appcat/apis/appcat/v1"
	"github.com/vshn/appcat/pkg/apiserver/appcat"
	"github.com/vshn/appcat/pkg/apiserver/vshn/postgres"
	"log"
	"os"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"
	"strconv"
)

var apiServerCMDStr = "apiserver"

var APIServerCMD = newAPIServerCMD()

func newAPIServerCMD() *cobra.Command {
	var appcatEnabled, vshnBackupsEnabled bool

	if len(os.Args) < 2 {
		return &cobra.Command{}
	}

	if os.Args[1] == apiServerCMDStr {
		appcatEnabled, vshnBackupsEnabled = parseEnvVariables()
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
	cmd.Use = "apiserver"
	cmd.Short = "AppCat API Server"
	cmd.Long = "Run the AppCat API Server"
	if err != nil {
		log.Fatal(err, "Unable to load build API Server")
	}
	return cmd
}

func parseEnvVariables() (bool, bool) {
	appcatEnabled, err := strconv.ParseBool(os.Getenv("APPCAT_HANDLER_ENABLED"))
	if err != nil {
		log.Fatal("Can't parse APPCAT_HANDLER_ENABLED env variable")
	}
	vshnBackupsEnabled, err := strconv.ParseBool(os.Getenv("VSHN_POSTGRES_BACKUP_HANDLER_ENABLED"))
	if err != nil {
		log.Fatal("Can't parse APPCAT_HANDLER_ENABLED env variable")
	}

	if !appcatEnabled && !vshnBackupsEnabled {
		log.Fatal("Handlers are not enabled, please set at least one of APPCAT_HANDLER_ENABLED | VSHN_POSTGRES_BACKUP_HANDLER_ENABLED env variables to True")
	}
	return appcatEnabled, vshnBackupsEnabled
}
