package cmd

import (
	"github.com/crossplane/function-sdk-go"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/buckets/cloudscalebucket"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/buckets/exoscalebucket"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/buckets/miniobucket"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/spksmariadb"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/spksredis"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnforgejo"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnkeycloak"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnmariadb"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnminio"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnnextcloud"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgres"
	_ "github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnredis"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	viper.AutomaticEnv()

	FunctionCMD.Flags().StringVar(&network, "network", "tcp", "network type")
	FunctionCMD.Flags().StringVar(&address, "address", ":9443", "set where socket should be located")
	FunctionCMD.Flags().BoolVar(&insecure, "insecure", false, "disable tls transport")
	FunctionCMD.Flags().StringVar(&tlsCertsDir, "certsdir", viper.GetString("TLS_SERVER_CERTS_DIR"), "Directory containing server certs (tls.key, tls.crt) and the CA used to verify client certificates (ca.crt) (env: TLS_SERVER_CERTS_DIR)")
	FunctionCMD.Flags().BoolVar(&proxyMode, "proxymode", false, "Enable proxy mode. If enabled the grpc calls will be forwarded to another GRPC endpoint.")
}

var (
	network     string
	address     string
	insecure    bool
	tlsCertsDir string
	proxyMode   bool
)

var FunctionCMD = &cobra.Command{
	Use:   "functions",
	Short: "Crossplane Functions Server",
	Long:  "Run the GRPC Server for crossplane composition functions",
	RunE:  executeFunctionsServer,
}

// Run will run the controller mode of the composition function runner.
func executeFunctionsServer(cmd *cobra.Command, _ []string) error {
	log := logr.FromContextOrDiscard(cmd.Context())
	ctrl.SetLogger(log)

	manager := runtime.NewManager(log, proxyMode)

	log.Info("Listening for grpc calls", "address", address, "network", network, "insecure", insecure, "proxy", proxyMode)

	return function.Serve(manager,
		function.Listen(network, address),
		function.MTLSCertificates(tlsCertsDir),
		function.Insecure(insecure))

}
