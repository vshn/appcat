package cmd

import (
	"context"
	pb "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	vpf "github.com/vshn/appcat-apiserver/pkg/comp-functions/functions/vshn-postgres-func"
	"github.com/vshn/appcat-apiserver/pkg/comp-functions/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	GrpcCMD.Flags().StringVar(&Network, "network", "unix", "network type")
	GrpcCMD.Flags().StringVar(&AddressFlag, "socket", "@crossplane/fn/default.sock", "set where socket should be located")
}

var (
	Network     = "unix"
	AddressFlag = "@crossplane/fn/default.sock"
)

var GrpcCMD = &cobra.Command{
	Use:   "start grpc",
	Short: "GRPC Server",
	Long:  "Run the GRPC Server for crossplane composition functions",
	RunE:  executeGRPCServer,
}

var postgresFunctions = []runtime.Transform{
	{
		Name:          "url-connection-details",
		TransformFunc: vpf.AddUrlToConnectionDetails,
	},
	{
		Name:          "user-alerting",
		TransformFunc: vpf.AddUserAlerting,
	},
	{
		Name:          "random-default-schedule",
		TransformFunc: vpf.TransformSchedule,
	},
	{
		Name:          "encrypted-pvc-secret",
		TransformFunc: vpf.AddPvcSecret,
	},
}

type server struct {
	pb.UnimplementedContainerizedFunctionRunnerServiceServer
	logger logr.Logger
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

func (s *server) RunFunction(ctx context.Context, in *pb.RunFunctionRequest) (*pb.RunFunctionResponse, error) {
	ctx = logr.NewContext(ctx, s.logger)
	switch in.Image {
	case "postgresql":
		fnio, err := runtime.RunCommand(ctx, in.Input, postgresFunctions)
		if err != nil {
			return &pb.RunFunctionResponse{
				Output: fnio,
			}, status.Errorf(codes.Aborted, "Can't process request for PostgreSQL")
		}
		return &pb.RunFunctionResponse{
			Output: fnio,
		}, nil
	case "redis":
		return &pb.RunFunctionResponse{
			// return what was sent as it's currently not supported
			Output: in.Input,
		}, status.Error(codes.Unimplemented, "Redis is not yet implemented")
	default:
		return &pb.RunFunctionResponse{
			Output: []byte("Bad configuration"),
		}, status.Error(codes.NotFound, "Unknown request")
	}
}

// socket isn't removed after server stop listening and blocks another starts
func cleanStart(socketName string) error {
	if _, err := os.Stat(socketName); err == nil {
		err = os.RemoveAll(socketName)
		return err
	}

	return nil
}
