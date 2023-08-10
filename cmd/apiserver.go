package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	appcatv1 "github.com/vshn/appcat/v4/apis/appcat/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver/appcat"
	vshnpostgres "github.com/vshn/appcat/v4/pkg/apiserver/vshn/postgres"
	vshnredis "github.com/vshn/appcat/v4/pkg/apiserver/vshn/redis"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"
)

var apiServerCMDStr = "apiserver"

var APIServerCMD = newAPIServerCMD()

func newAPIServerCMD() *cobra.Command {

	viper.AutomaticEnv()

	var appcatEnabled, vshnPGBackupsEnabled, vshnRedisBackupsEnabled bool

	if len(os.Args) < 2 {
		return &cobra.Command{}
	}

	if os.Args[1] == apiServerCMDStr {
		appcatEnabled = viper.GetBool("APPCAT_HANDLER_ENABLED")
		vshnPGBackupsEnabled = viper.GetBool("VSHN_POSTGRES_BACKUP_HANDLER_ENABLED")
		vshnRedisBackupsEnabled = viper.GetBool("VSHN_REDIS_BACKUP_HANDLER_ENABLED")
		if !appcatEnabled && !vshnPGBackupsEnabled && !vshnRedisBackupsEnabled {
			log.Fatal("Handlers are not enabled, please set at least one of APPCAT_HANDLER_ENABLED | VSHN_POSTGRES_BACKUP_HANDLER_ENABLED | VSHN_REDIS_BACKUP_HANDLER_ENABLED env variables to True")
		}
	}

	b := builder.APIServer

	if appcatEnabled {
		b.WithResourceAndHandler(&appcatv1.AppCat{}, appcat.New())
	}

	if vshnPGBackupsEnabled {
		b.WithResourceAndHandler(&appcatv1.VSHNPostgresBackup{}, vshnpostgres.New())
	}

	if vshnRedisBackupsEnabled {
		b.WithResourceAndHandler(&appcatv1.VSHNRedisBackup{}, vshnredis.New())
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
