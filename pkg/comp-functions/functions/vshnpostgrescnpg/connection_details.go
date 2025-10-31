package vshnpostgrescnpg

import (
	"context"
	"fmt"
	"strings"

	// "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

const (
	ClusterInstanceCdField = "clusterInstances"
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
)

func AddConnectionSecrets(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	if err := writeConnectionDetailsToSvc(svc, comp); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("Couldn't set connection details: %v", err))
	}

	return nil
}

func generateConnectionDetailInfoForRelease(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) []v1beta1.ConnectionDetail {
	var connectionDetails []v1beta1.ConnectionDetail

	// PSQL credentials
	for secretKey, toCdField := range map[string]string{
		"uri":      PostgresqlURL,
		"user":     PostgresqlDb,
		"password": PostgresqlPassword,
		"port":     PostgresqlPort,
		"username": PostgresqlUser,
		"host":     PostgresqlHost,
	} {
		connectionDetails = append(connectionDetails, v1beta1.ConnectionDetail{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       comp.GetName() + "-cluster-superuser",
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  fmt.Sprintf("data.%s", secretKey),
			},
			ToConnectionSecretKey:  toCdField,
			SkipPartOfReleaseCheck: true,
			// This secret gets created by the Cluster CR and is not templated by the release
		})
	}

	// Certificate
	for _, secretKey := range []string{
		"ca.crt",
		"tls.crt",
		"tls.key",
	} {
		connectionDetails = append(connectionDetails, v1beta1.ConnectionDetail{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "tls-certificate",
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  fmt.Sprintf("data[%s]", secretKey),
			},
			ToConnectionSecretKey:  secretKey,
			SkipPartOfReleaseCheck: true,
			// This secret gets created by a custom Certificate CR
		})
	}

	// Cluster watch
	connectionDetails = append(connectionDetails, v1beta1.ConnectionDetail{
		ObjectReference: corev1.ObjectReference{
			APIVersion: "postgresql.cnpg.io/v1",
			Kind:       "Cluster",
			Name:       comp.GetName() + "-cluster",
			Namespace:  comp.GetInstanceNamespace(),
			FieldPath:  "status.instanceNames",
		},
		ToConnectionSecretKey: ClusterInstanceCdField,
	})

	return connectionDetails
}

func writeConnectionDetailsToSvc(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) error {
	cd, err := getConnectionDetails(svc, comp)
	if err != nil {
		return fmt.Errorf("could not write connection details to svc: %w", err)
	}

	for k, v := range cd {
		svc.SetConnectionDetail(k, v)
	}

	return nil
}

// Get CNPG cluster instances as reported by the connection details of the release
func getClusterInstancesReportedByCd(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) ([]string, error) {
	cd, err := getConnectionDetails(svc, comp)
	if err != nil {
		return nil, err
	}

	cdValue, exists := cd[ClusterInstanceCdField]
	if !exists {
		return nil, fmt.Errorf("cluster instances not known in connection details")
	}

	// cdValue will be a []byte of a literal string of an array such as "[a b c]", so we need to convert it into an actual []string first.
	trimmed := strings.Trim(string(cdValue), "\n\r\t") // <- Unlikely to be present in the CD, but will accommodate tests.
	trimmed = strings.Trim(trimmed, "[]")

	return strings.Fields(trimmed), nil
}

func getConnectionDetails(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) (map[string][]byte, error) {
	cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName())
	if err != nil {
		return nil, fmt.Errorf("could not get connection details: %w", err)
	}

	if len(cd) <= 0 {
		return nil, fmt.Errorf("connection details not (yet) populated")
	}

	return cd, nil
}
