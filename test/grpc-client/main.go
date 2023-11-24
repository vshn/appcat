/*

Client is just for debugging purposes, it sends the same request as crossplane would send

*/

package main

// pb "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"

var (
	Network = "unix"
	Address = "../../default.sock"
)

// func main() {
// 	// reuse the same output as we use in tests
// 	// You must run this file from it's directory or adjust file path
// 	b1, err := os.ReadFile("../../test/example.yaml")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", Network, Address), grpc.WithTransportCredentials(insecure.NewCredentials()))
// 	if err != nil {
// 		log.Fatalf("did not connect: %v", err)
// 	}
// 	client := pb.NewContainerizedFunctionRunnerServiceClient(conn)
// 	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
// 	defer cancel()

// 	sample_request := pb.RunFunctionRequest{
// 		Input: b1,
// 		Image: "postgresql",
// 	}

// 	resp, err := client.RunFunction(ctx, &sample_request)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	fmt.Println(string(resp.GetOutput()))
// }
