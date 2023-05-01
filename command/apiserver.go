package command

import (
	"github.com/spf13/cobra"
	appcatv1 "github.com/vshn/appcat-apiserver/apis/appcat/v1"
	"github.com/vshn/appcat-apiserver/apiserver/appcat"
	"github.com/vshn/appcat-apiserver/apiserver/vshn/postgres"
	"log"
	"os"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"
	"strconv"
)

var apiServerCMDStr = "apiserver"

var apiServerCmd = newAPIServerCMD()

func newAPIServerCMD() *cobra.Command {
	var appcatEnabled, vshnBackupsEnabled bool

	if os.Args[1] == apiServerCMDStr {
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
