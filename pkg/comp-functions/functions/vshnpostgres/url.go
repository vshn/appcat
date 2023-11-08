package vshnpostgres

import (
	"context"
	"fmt"

	// "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	"github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var (
	// PostgresqlHost is env variable in the connection secret
	PostgresqlHost = "POSTGRESQL_HOST"
	// PostgresqlUser is env variable in the connection secret
	PostgresqlUser = "POSTGRESQL_USER"
	// PostgresqlPassword is env variable in the connection secret
	PostgresqlPassword = "POSTGRESQL_PASSWORD"
	// PostgresqlPort is env variable in the connection secret
	PostgresqlPort = "POSTGRESQL_PORT"
	// PostgresqlDb is env variable in the connection secret
	PostgresqlDb = "POSTGRESQL_DB"
	// PostgresqlUrl is env variable in the connection secret
	PostgresqlUrl = "POSTGRESQL_URL"
)

// connectionSecretResourceName is the resource name defined in the composition
// This name is different from metadata.name of the same resource
// The value is hardcoded in the composition for each resource and due to crossplane limitation
// it cannot be matched to the metadata.name
var connectionSecretResourceName = "connection"

// AddUrlToConnectionDetails changes the desired state of a FunctionIO
func AddUrlToConnectionDetails(ctx context.Context, svc *runtime.ServiceRuntime) *v1beta1.Result {
	log := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	// Wait for the next reconciliation in case instance namespace is missing
	if comp.Status.InstanceNamespace == "" {
		return runtime.NewWarningResult("Composite is missing instance namespace, skipping transformation")
	}

	log.Info("Getting connection secret from managed kubernetes object")
	s := &v1.Secret{}

	err = svc.GetObservedKubeObject(s, connectionSecretResourceName)
	if err != nil {
		return runtime.NewWarningResult("Cannot get connection secret object")
	}

	log.Info("Setting POSTRESQL_URL env variable into connection secret")
	val := getPostgresURL(s)
	if val == "" {
		return runtime.NewWarningResult("User, pass, host, port or db value is missing from connection secret, skipping transformation")
	}

	svc.SetConnectionDetail(PostgresqlUrl, []byte(val))

	return nil
}

func getPostgresURL(s *v1.Secret) string {
	user := string(s.Data[PostgresqlUser])
	pwd := string(s.Data[PostgresqlPassword])
	host := string(s.Data[PostgresqlHost])
	port := string(s.Data[PostgresqlPort])
	db := string(s.Data[PostgresqlDb])

	// The values are still missing, wait for the next reconciliation
	if user == "" || pwd == "" || host == "" || port == "" || db == "" {
		return ""
	}

	return "postgres://" + user + ":" + pwd + "@" + host + ":" + port + "/" + db
}
