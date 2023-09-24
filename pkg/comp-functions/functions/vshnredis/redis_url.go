package vshnredis

import (
	"context"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
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
func AddUrlToConnectionDetails(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	log := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNRedis{}
	err := iof.Desired.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get composite from function io", err)
	}

	// Wait for the next reconciliation in case instance namespace is missing
	if comp.Status.InstanceNamespace == "" {
		return runtime.NewWarning(ctx, "Composite is missing instance namespace, skipping transformation")
	}

	log.Info("Getting connection secret from managed kubernetes object")
	s := &v1.Secret{}

	err = iof.Observed.GetFromObject(ctx, s, connectionSecretResourceName)
	if err != nil {
		return runtime.NewWarning(ctx, "Cannot get connection secret object")
	}

	log.Info("Setting REDIS_URL env variable into connection secret")
	val := getRedisURL(s)
	if val == "" {
		return runtime.NewWarning(ctx, "User, pass, host or port value is missing from connection secret, skipping transformation")
	}
	iof.Desired.PutCompositeConnectionDetail(ctx, v1alpha1.ExplicitConnectionDetail{
		Name:  RedisUrl,
		Value: val,
	})

	return runtime.NewNormal()
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
