package cmd

import (
	"context"
	"net"
	"os"

	pb "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/helper"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/miniobucket"
	vpf "github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshn-postgres-func"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnminio"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnredis"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	GrpcCMD.Flags().StringVar(&Network, "network", "unix", "network type")
	GrpcCMD.Flags().StringVar(&AddressFlag, "socket", "@crossplane/fn/default.sock", "set where socket should be located")
	GrpcCMD.Flags().BoolVar(&DevMode, "devmode", false, "enable dev composition functions")
}

var (
	Network     = "unix"
	AddressFlag = "@crossplane/fn/default.sock"
	DevMode     = false
)

var GrpcCMD = &cobra.Command{
	Use:   "start grpc",
	Short: "GRPC Server",
	Long:  "Run the GRPC Server for crossplane composition functions",
	RunE:  executeGRPCServer,
}

// Run will run the controller mode of the composition function runner.
func executeGRPCServer(cmd *cobra.Command, _ []string) error {
	log := logr.FromContextOrDiscard(cmd.Context())
	ctrl.SetLogger(log)

	if err := cleanStart(AddressFlag); err != nil {
		log.Error(err, "cannot clean start GRPC server.")
		return err
	}

	lis, err := net.Listen(Network, AddressFlag)
	if err != nil {
		log.Error(err, "failed to listen.")
		return err
	}

	s := grpc.NewServer()

	pb.RegisterContainerizedFunctionRunnerServiceServer(s, &server{logger: log})
	log.Info("server listening at", "address", lis.Addr())
	if err = s.Serve(lis); err != nil {
		log.Error(err, "failed to serve.")
		return err
	}
	return nil
}

var images = map[string][]runtime.Transform{
	"postgresql": {
		{
			Name:          "url-connection-details",
			TransformFunc: vpf.AddUrlToConnectionDetails,
		},
		{
			Name:          "user-alerting",
			TransformFunc: vpf.AddUserAlerting,
		},
		{
			Name:          "restart",
			TransformFunc: vpf.TransformRestart,
		},
		{
			Name:          "random-default-schedule",
			TransformFunc: vpf.TransformSchedule,
		},
		{
			Name:          "encrypted-pvc-secret",
			TransformFunc: vpf.AddPvcSecret,
		},
		{
			Name:          "maintenance-job",
			TransformFunc: vpf.AddMaintenanceJob,
		},
		{
			Name:          "mailgun-alerting",
			TransformFunc: vpf.MailgunAlerting,
		},
		{
			Name:          "extensions",
			TransformFunc: vpf.AddExtensions,
		},
		{
			Name:          "replication",
			TransformFunc: vpf.ConfigureReplication,
		},
		{
			Name:          "load-balancer",
			TransformFunc: vpf.AddLoadBalancerIPToConnectionDetails,
		},
		{
			Name:          "namespaceQuotas",
			TransformFunc: common.AddInitialNamespaceQuotas("namespace-conditions"),
		},
	},
	"redis": {
		{
			Name:          "manage-release",
			TransformFunc: vshnredis.ManageRelease,
		},
		{
			Name:          "backup",
			TransformFunc: vshnredis.AddBackup,
		},
		{
			Name:          "restore",
			TransformFunc: vshnredis.RestoreBackup,
		},
		{
			Name:          "maintenance",
			TransformFunc: vshnredis.AddMaintenanceJob,
		},
		{
			Name:          "resizePVC",
			TransformFunc: vshnredis.ResizePVCs,
		},
		{
			Name:          "namespaceQuotas",
			TransformFunc: common.AddInitialNamespaceQuotas("namespace-conditions"),
		},
		{
			Name:          "redis_url",
			TransformFunc: vshnredis.AddUrlToConnectionDetails,
		},
	},
	"minio": {
		{
			Name:          "deploy",
			TransformFunc: vshnminio.DeployMinio,
		},
		{
			Name:          "deploy-providerconfig",
			TransformFunc: vshnminio.DeployMinioProviderConfig,
		},
		{
			Name:          "maintenance",
			TransformFunc: vshnminio.AddMaintenanceJob,
		},
	},
	"miniobucket": {
		{
			Name:          "provision-bucket",
			TransformFunc: miniobucket.ProvisionMiniobucket,
		},
	},
}

type server struct {
	pb.UnimplementedContainerizedFunctionRunnerServiceServer
	logger logr.Logger
}

func (s *server) RunFunction(ctx context.Context, in *pb.RunFunctionRequest) (*pb.RunFunctionResponse, error) {
	ctx = logr.NewContext(ctx, s.logger)
	err := enableDevMode(DevMode)
	if err != nil {
		return nil, status.Errorf(codes.Aborted, "cannot enable devMode: %s", err)
	}

	fnio, err := runtime.RunCommand(ctx, in.Input, images[in.Image])
	if err != nil {
		err = status.Errorf(codes.Aborted, "Can't process request for PostgreSQL")
	}
	return &pb.RunFunctionResponse{Output: fnio}, err
}

// socket isn't removed after server stop listening and blocks another starts
func cleanStart(socketName string) error {
	if _, err := os.Stat(socketName); err == nil {
		err = os.RemoveAll(socketName)
		return err
	}

	return nil
}

func enableDevMode(enable bool) error {

	kubeClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return err
	}

	if enable {
		for key := range images {
			found := false
			for _, transform := range images[key] {
				if transform.Name == "saveFunctionIO" {
					found = true
					break
				}
			}
			if !found {
				images[key] = append(images[key], runtime.Transform{
					Name:          "saveFunctionIO",
					TransformFunc: helper.SaveFuncIOToConfigMap(key, kubeClient),
				})
			}
		}
	}
	return nil
}
