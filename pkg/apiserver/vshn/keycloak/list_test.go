package keycloak

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_vshnKeycloakBackupStorage_getPostgreSQLNamespaceAndName(t *testing.T) {
	fclient := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).
		WithObjects().Build()

	keycloakStorage := vshnKeycloakBackupStorage{
		vshnKeycloak: &concreteKeycloakProvider{
			ClientConfigurator: apiserver.New(fclient),
		},
	}

	// Given
	comp := &vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Status: vshnv1.XVSHNPostgreSQLStatus{
			VSHNPostgreSQLStatus: vshnv1.VSHNPostgreSQLStatus{
				InstanceNamespace: "test-ns",
			},
		},
	}

	assert.NoError(t, fclient.Create(context.TODO(), comp))

	claim := &vshnv1.VSHNKeycloak{
		Spec: vshnv1.VSHNKeycloakSpec{
			ResourceRef: xpv1.TypedReference{
				Name: "test-nc-comp",
			},
		},
	}

	kcComp := &vshnv1.XVSHNKeycloak{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-nc-comp",
		},
		Spec: vshnv1.XVSHNKeycloakSpec{
			ResourceRefs: []xpv1.TypedReference{
				{
					Name: "test",
					Kind: "XVSHNPostgreSQL",
				},
			},
		},
	}

	assert.NoError(t, fclient.Create(context.TODO(), kcComp))

	// When
	namespace, name := keycloakStorage.getPostgreSQLNamespaceAndName(context.TODO(), claim)

	// Then
	assert.Equal(t, "test", name)
	assert.Equal(t, "test-ns", namespace)
}
