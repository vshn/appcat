package vshnpostgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	pgv1alpha1 "github.com/vshn/appcat/v4/apis/sql/postgresql/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func Test_addProviderConfig(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))
	addProviderConfig(comp, svc, comp.Spec.Parameters.Service.TLS.Enabled)

	// then
	secret := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(secret, comp.GetName()+"-provider-conf-credentials"))

	config := &pgv1alpha1.ProviderConfig{}
	assert.NoError(t, svc.GetDesiredKubeObject(config, comp.GetName()+"-providerconfig"))
	assert.Equal(t, *config.Spec.SSLMode, "require")

}

func Test_tlsDisabled(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/02-tls-disabled.yaml")

	// when
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))
	addProviderConfig(comp, svc, comp.Spec.Parameters.Service.TLS.Enabled)

	// then
	secret := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(secret, comp.GetName()+"-provider-conf-credentials"))

	config := &pgv1alpha1.ProviderConfig{}
	assert.NoError(t, svc.GetDesiredKubeObject(config, comp.GetName()+"-providerconfig"))
	assert.Equal(t, *config.Spec.SSLMode, "disable")

}

func Test_addUser(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	addUser(comp, svc, "unit")

	// then
	role := &pgv1alpha1.Role{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(role, fmt.Sprintf("%s-%s-role", comp.GetName(), "unit")))

}

func Test_addDatabase(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	addDatabase(comp, svc, "unit")

	// then
	db := &pgv1alpha1.Database{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "unit")))

}

func Test_addGrants(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	addGrants(comp, svc, "unit", "unit", []string{"ALL"})

	// then
	grant := &pgv1alpha1.Grant{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(grant, fmt.Sprintf("%s-%s-%s-grants", comp.GetName(), "unit", "unit")))

}

func TestUserManagement(t *testing.T) {
	// given with empty accesss object
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when applied
	assert.Nil(t, UserManagement(context.TODO(), &vshnv1.VSHNPostgreSQL{}, svc))

	// then expect no database
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	db := &pgv1alpha1.Database{}
	assert.Error(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "prod")))

	// when adding an user
	comp.Spec.Parameters.Service.Access = []vshnv1.VSHNAccess{
		{
			User: ptr.To("prod"),
		},
	}

	assert.NoError(t, svc.SetDesiredCompositeStatus(comp))
	assert.Nil(t, UserManagement(context.TODO(), &vshnv1.VSHNPostgreSQL{}, svc))

	// then expect database
	db = &pgv1alpha1.Database{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "prod")))

	// when adding user pointing to same db
	comp.Spec.Parameters.Service.Access = append(comp.Spec.Parameters.Service.Access, vshnv1.VSHNAccess{
		User:     ptr.To("another"),
		Database: ptr.To("prod"),
	})

	assert.NoError(t, svc.SetDesiredCompositeStatus(comp))
	assert.Nil(t, UserManagement(context.TODO(), &vshnv1.VSHNPostgreSQL{}, svc))

	// then expect database
	db = &pgv1alpha1.Database{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "prod")))

	// When deleting user pointing to same db
	comp.Spec.Parameters.Service.Access = []vshnv1.VSHNAccess{
		{
			User:     ptr.To("another"),
			Database: ptr.To("prod"),
		},
	}

	// then expect database
	db = &pgv1alpha1.Database{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "prod")))
}
