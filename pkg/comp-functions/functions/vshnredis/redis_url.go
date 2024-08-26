package vshnredis

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	// RedisHost is env variable in the connection secret
	RedisHost = "REDIS_HOST"
	// RedisUser is env variable in the connection secret
	RedisUser = "REDIS_USERNAME"
	// RedisPassword is env variable in the connection secret
	RedisPassword = "REDIS_PASSWORD"
	// RedisPort is env variable in the connection secret
	RedisPort = "REDIS_PORT"
	// RedisURL is env variable in the connection secret
	RedisURL = "REDIS_URL"
)

// AddUrlToConnectionDetails changes the desired state of a FunctionIO
func AddUrlToConnectionDetails(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	host := fmt.Sprintf("redis-headless.vshn-redis-%s.svc.cluster.local", comp.GetName())
	port := "6379"
	user := "default"

	cd := svc.GetConnectionDetails()

	log.Info("Setting REDIS_URL env variable into connection secret")
	val := getRedisURL(cd, host, port, user)
	if val == "" {
		return runtime.NewWarningResult("User, pass, host or port value is missing from connection secret, skipping transformation")
	}

	svc.SetConnectionDetail(RedisUser, []byte(user))
	svc.SetConnectionDetail(RedisPort, []byte(port))
	svc.SetConnectionDetail(RedisHost, []byte(host))
	svc.SetConnectionDetail(RedisURL, []byte(val))

	return nil
}

func getRedisURL(cd map[string][]byte, host, port, user string) string {
	pwd := string(cd[RedisPassword])

	// The values are still missing, wait for the next reconciliation
	if user == "" || pwd == "" || host == "" || port == "" {
		return ""
	}

	return "rediss://" + user + ":" + pwd + "@" + host + ":" + port
}
