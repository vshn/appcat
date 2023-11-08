package vshnredis

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var (
	// RedisHost is env variable in the connection secret
	RedisHost = "REDIS_HOST"
	// RedisUser is env variable in the connection secret
	RedisUser = "REDIS_USERNAME"
	// RedisPassword is env variable in the connection secret
	RedisPassword = "REDIS_PASSWORD"
	// RedisPort is env variable in the connection secret
	RedisPort = "REDIS_PORT"
	// RedisUrl is env variable in the connection secret
	RedisUrl = "REDIS_URL"
)

// connectionSecretResourceName is the resource name defined in the composition
// This name is different from metadata.name of the same resource
// The value is hardcoded in the composition for each resource and due to crossplane limitation
// it cannot be matched to the metadata.name
var connectionSecretResourceName = "connection"

// AddUrlToConnectionDetails changes the desired state of a FunctionIO
func AddUrlToConnectionDetails(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	log.Info("Getting connection secret from managed kubernetes object")
	s := &v1.Secret{}

	err = svc.GetObservedKubeObject(s, connectionSecretResourceName)
	if err != nil {
		return runtime.NewWarningResult("Cannot get connection secret object")
	}

	log.Info("Setting REDIS_URL env variable into connection secret")
	val := getRedisURL(s)
	if val == "" {
		return runtime.NewWarningResult("User, pass, host or port value is missing from connection secret, skipping transformation")
	}

	svc.SetConnectionDetail(RedisUrl, []byte(val))

	return nil
}

func getRedisURL(s *v1.Secret) string {
	user := string(s.Data[RedisUser])
	pwd := string(s.Data[RedisPassword])
	host := string(s.Data[RedisHost])
	port := string(s.Data[RedisPort])

	// The values are still missing, wait for the next reconciliation
	if user == "" || pwd == "" || host == "" || port == "" {
		return ""
	}

	return "rediss://" + user + ":" + pwd + "@" + host + ":" + port
}
