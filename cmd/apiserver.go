package cmd

import (
	"log"

	"github.com/spf13/cobra"
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver/appcat"
	vshnkeycloak "github.com/vshn/appcat/v4/pkg/apiserver/vshn/keycloak"
	vshnmariadb "github.com/vshn/appcat/v4/pkg/apiserver/vshn/mariadb"
	vshnnextcloud "github.com/vshn/appcat/v4/pkg/apiserver/vshn/nextcloud"
	vshnpostgres "github.com/vshn/appcat/v4/pkg/apiserver/vshn/postgres"
	vshnredis "github.com/vshn/appcat/v4/pkg/apiserver/vshn/redis"
	appcatopenapi "github.com/vshn/appcat/v4/pkg/openapi"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"
)

var APIServerCMD = newAPIServerCMD()

func newAPIServerCMD() *cobra.Command {

	b := builder.APIServer

	b.WithResourceAndHandler(&appcatv1.AppCat{}, appcat.New())

	b.WithResourceAndHandler(&appcatv1.VSHNPostgresBackup{}, vshnpostgres.New())

	b.WithResourceAndHandler(&appcatv1.VSHNRedisBackup{}, vshnredis.New())

	b.WithResourceAndHandler(&appcatv1.VSHNMariaDBBackup{}, vshnmariadb.New())

	b.WithResourceAndHandler(&appcatv1.VSHNNextcloudBackup{}, vshnnextcloud.New())

	b.WithResourceAndHandler(&appcatv1.VSHNKeycloakBackup{}, vshnkeycloak.New())

	b.WithoutEtcd().
		ExposeLoopbackAuthorizer().
		ExposeLoopbackMasterClientConfig()

	b.WithOpenAPIDefinitions("Wardle", "0.1", appcatopenapi.GetOpenAPIDefinitions)

	cmd, err := b.Build()
	cmd.Use = "apiserver"
	cmd.Short = "AppCat API Server"
	cmd.Long = "Run the AppCat API Server"
	if err != nil {
		log.Fatal(err, "Unable to load build API Server")
	}

	return cmd
}
