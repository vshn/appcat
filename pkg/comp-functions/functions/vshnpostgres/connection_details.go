package vshnpostgres

import (
	"context"
	"fmt"

	// "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/function-sdk-go/proto/v1"
	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
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
	// PostgresqlURL is env variable in the connection secret
	PostgresqlURL = "POSTGRESQL_URL"
	defaultUser   = "postgres"
	defaultPort   = "5432"
	defaultDB     = "postgres"
)

// AddConnectionDetails changes the desired state of a FunctionIO
func AddConnectionDetails(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *v1.Result {
	log := controllerruntime.LoggerFrom(ctx)

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	log.Info("Making sure the cluster exposed connection details")
	obj := &xkubev1.Object{}
	err = svc.GetDesiredComposedResourceByName(obj, "cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get the sgcluster object: %s", err))
	}

	err = addConnectionDetailsToObject(obj, comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot expose connection details on cluster: %s", err))
	}

	log.Info("Creating connection details")
	cd, err := svc.GetObservedComposedResourceConnectionDetails("cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get credentials from cluster object: %s", err))
	}

	if len(cd) == 0 {
		return runtime.NewWarningResult("no connection details yet on cluster")
	}

	rootPw, err := getPGRootPassword(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("cannot observe root password: " + err.Error())
	}

	host := fmt.Sprintf("%s.vshn-postgresql-%s.svc.cluster.local", comp.GetName(), comp.GetName())

	url := getPostgresURL(host, rootPw)

	svc.SetConnectionDetail(PostgresqlURL, []byte(url))
	svc.SetConnectionDetail(PostgresqlDb, []byte(defaultDB))
	svc.SetConnectionDetail(PostgresqlPort, []byte(defaultPort))
	svc.SetConnectionDetail(PostgresqlPassword, []byte(rootPw))
	svc.SetConnectionDetail(PostgresqlUser, []byte(defaultUser))
	svc.SetConnectionDetail(PostgresqlHost, []byte(host))
	err = svc.AddObservedConnectionDetails("cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add connection details to composite: %s", err))
	}

	return nil
}

func getPostgresURL(host, pw string) string {
	return getPostgresURLCustomUser(host, defaultUser, pw, defaultDB)
}

func getPostgresURLCustomUser(host, userName, pw, db string) string {
	// The values are still missing, wait for the next reconciliation
	if pw == "" {
		return ""
	}

	return "postgres://" + userName + ":" + pw + "@" + host + ":" + defaultPort + "/" + db
}

func addConnectionDetailsToObject(obj *xkubev1.Object, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	certSecretName := "tls-certificate"

	obj.Spec.ConnectionDetails = []xkubev1.ConnectionDetail{
		{
			ToConnectionSecretKey: "ca.crt",
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  comp.GetInstanceNamespace(),
				Name:       certSecretName,
				FieldPath:  "data[ca.crt]",
			},
		},
		{
			ToConnectionSecretKey: "tls.crt",
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  comp.GetInstanceNamespace(),
				Name:       certSecretName,
				FieldPath:  "data[tls.crt]",
			},
		},
		{
			ToConnectionSecretKey: "tls.key",
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  comp.GetInstanceNamespace(),
				Name:       certSecretName,
				FieldPath:  "data[tls.key]",
			},
		},
	}

	obj.Spec.WriteConnectionSecretToReference = &commonv1.SecretReference{
		Name:      comp.GetName() + "-connection",
		Namespace: svc.GetCrossplaneNamespace(),
	}

	err := svc.SetDesiredComposedResourceWithName(obj, "cluster")
	if err != nil {
		return fmt.Errorf("cannot deploy postgresql connection details: %w", err)
	}

	return nil
}

// getPGRootPassword will deploy an observer for stackgres' generated secret and return the password for the root user.
// This is necessary, because provider-kubernetes can hang during de-provisioning, if the secret is used as a connectiondetails
// reference. During deletion, if the secret gets removed before the kube-object gets removed, the kube-object will get stuck
// with observation errors, as it can't resolve the connectiondetails anymore. This is a bug in provider-kubernetes itself.
// To avoid this, we deploy a separate observer for that secret and get the value directly that way.
func getPGRootPassword(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) (string, error) {
	resNameSuffix := "-root-pw-observer"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
	}

	err := svc.SetDesiredKubeObject(secret, comp.GetName()+resNameSuffix, runtime.KubeOptionObserve)
	if err != nil {
		return "", err
	}

	err = svc.GetObservedKubeObject(secret, comp.GetName()+resNameSuffix)
	if err != nil {
		if err == runtime.ErrNotFound {
			return "", nil
		}
		return "", err
	}

	pw := secret.Data["superuser-password"]

	return string(pw), nil
}
