package vshnpostgrescnpg

import (
	"fmt"
	"strings"

	// "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"

	"github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

const (
	ClusterInstanceCdField = "clusterInstances"
)

func generateConnectionDetailsForRelease(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) []v1beta1.ConnectionDetail {
	var connectionDetails []v1beta1.ConnectionDetail

	// PSQL credentials
	for secretKey, toCdField := range map[string]string{
		"uri":      "POSTGRESQL_URL",
		"user":     "POSTGRESQL_DB",
		"password": "POSTGRESQL_PASSWORD",
		"port":     "POSTGRESQL_PORT",
		"username": "POSTGRESQL_USER",
		"host":     "POSTGRESQL_HOST",
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

// Get CNPG cluster instances as reported by the connection details of the release
func getClusterInstancesReportedByCd(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) ([]string, error) {
	// Whenever CNPG scales, new instances won't reuse a previous index.
	// Therefore, it's important to check what instances the Cluster CR is reporting.
	// We are proxying this field through the releases connection details in order to save on objects.
	cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName())
	if err != nil {
		return nil, fmt.Errorf("could not get connection details: %w", err)
	}

	if len(cd) <= 0 {
		return nil, fmt.Errorf("connection details not (yet) populated")
	}

	cdValue, exists := cd[ClusterInstanceCdField]
	if !exists {
		return nil, fmt.Errorf("cluster instances not known in connection details")
	}

	// cdValue will be a []byte of a literal string of an array such as "[a b c]", so we need to convert it into an actual []string first
	trimmed := strings.Trim(string(cdValue), "[]")
	return strings.Fields(trimmed), nil
}
