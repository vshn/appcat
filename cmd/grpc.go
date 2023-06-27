package cmd

import (
	"context"
	"net"
	"os"

	pb "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/vshn/appcat/pkg/comp-functions/functions/miniodev"
	vpf "github.com/vshn/appcat/pkg/comp-functions/functions/vshn-postgres-func"
	"github.com/vshn/appcat/pkg/comp-functions/functions/vshnredis"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"
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
	},
	"redis": {
		{
			Name:          "backup",
			TransformFunc: vshnredis.AddBackup,
		},
		{
			Name:          "maintenance",
			TransformFunc: vshnredis.AddMaintenanceJob,
		},
	},
}

type server struct {
	pb.UnimplementedContainerizedFunctionRunnerServiceServer
	logger logr.Logger
}

func (s *server) RunFunction(ctx context.Context, in *pb.RunFunctionRequest) (*pb.RunFunctionResponse, error) {
	ctx = logr.NewContext(ctx, s.logger)
	enableDevMode(DevMode)
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

func enableDevMode(enable bool) {
	if enable {
		images["miniodev"] = []runtime.Transform{
			{
				Name:          "miniodevbucket",
				TransformFunc: miniodev.ProvisionMiniobucket,
			},
		}
	}
}
