package vshnpostgrescnpg

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UserManagement(_ context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite from function io: %w", err))
	}

	if len(comp.Spec.Parameters.Service.Access) == 0 {
		return nil
	}

	for _, access := range comp.Spec.Parameters.Service.Access {
		dbname := *access.User
		if access.Database != nil {
			dbname = *access.Database
		}

		secretName := addCnpgUser(comp, svc, *access.User)

		err := addCnpgConnectionDetail(comp, svc, secretName, *access.User, dbname, access.WriteConnectionSecretToReference)
		if err != nil {
			return runtime.NewWarningResult("cannot add connection details: " + err.Error())
		}
	}

	return nil
}

// addCnpgUser creates a password secret in the instance namespace for the given username.
// The secret requires both "username" and "password" keys — CNPG's managed roles controller
// reads the referenced passwordSecret expecting both keys to be present.
// Returns the KubeObject resource name so callers can reference it later.
func addCnpgUser(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime, username string) string {
	secretName, err := common.AddGenericSecret(comp, svc, "userpass-"+username, []string{"password"}, common.AllowDeletion,
		common.AddStaticFieldToSecret(map[string]string{"username": username}))
	if err != nil {
		svc.Log.Error(err, "cannot deploy user password secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot deploy user password secret: %s", err)))
	}
	return secretName
}

// addCnpgConnectionDetail creates a user-facing connection secret in the claim namespace.
// It defers creation until the password secret is available in the observed state.
func addCnpgConnectionDetail(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime, secretName, username, dbname string, connectionDetailRef *xpv1.SecretReference) error {
	userpassCD, err := svc.GetObservedComposedResourceConnectionDetails(secretName)
	if err != nil {
		svc.Log.Error(err, "cannot get user password from secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot get user password from secret: %s", err)))
	}

	// Password not yet available — mark unready and skip; will retry on next reconcile.
	if len(userpassCD) == 0 {
		svc.SetDesiredResourceReadiness(secretName, runtime.ResourceUnReady)
		return nil
	}

	compositeCD := svc.GetConnectionDetails()

	host := string(compositeCD[PostgresqlHost])
	port := string(compositeCD[PostgresqlPort])
	url := buildPostgresURL(host, username, string(userpassCD["password"]), dbname)

	om := metav1.ObjectMeta{
		Name:      comp.GetLabels()["crossplane.io/claim-name"] + "-" + username,
		Namespace: comp.GetClaimNamespace(),
	}
	if connectionDetailRef != nil {
		om.Name = connectionDetailRef.Name
		om.Namespace = connectionDetailRef.Namespace
	}

	userpassSecret := &corev1.Secret{
		ObjectMeta: om,
		Type:       corev1.SecretType("connection.crossplane.io/v1alpha1"),
		Data: map[string][]byte{
			"POSTGRESQL_USER":     []byte(username),
			"POSTGRESQL_PASSWORD": userpassCD["password"],
			"POSTGRESQL_DB":       []byte(dbname),
			"POSTGRESQL_HOST":     []byte(host),
			"POSTGRESQL_PORT":     []byte(port),
			"POSTGRESQL_URL":      []byte(url),
			"ca.crt":              compositeCD["ca.crt"],
			"tls.crt":             compositeCD["tls.crt"],
			"tls.key":             compositeCD["tls.key"],
		},
	}

	err = svc.SetDesiredKubeObject(userpassSecret, fmt.Sprintf("%s-user-%s", comp.GetName(), username),
		runtime.KubeOptionAllowDeletion,
		runtime.KubeOptionDeployOnControlPlane)
	if err != nil {
		svc.Log.Error(err, "cannot set user connection secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot set user connection secret: %s", err)))
	}

	return nil
}

// buildPostgresURL constructs a postgres:// connection URL.
// Returns an empty string if the password is empty (not yet available).
func buildPostgresURL(host, user, password, db string) string {
	if password == "" {
		return ""
	}
	return "postgres://" + user + ":" + password + "@" + host + ":5432/" + db
}
