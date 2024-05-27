package vshnpostgres

import (
	"context"
	"fmt"
	"testing"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	apix "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/stretchr/testify/assert"
	pgv1alpha1 "github.com/vshn/appcat/v4/apis/sql/postgresql/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func Test_addProviderConfig(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))
	addProviderConfig(comp, svc)

	// then
	secret := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(secret, comp.GetName()+"-provider-conf-credentials"))

	config := &pgv1alpha1.ProviderConfig{}
	assert.NoError(t, svc.GetDesiredKubeObject(config, comp.GetName()+"-providerconfig"))
	assert.Equal(t, comp.GetInstanceNamespace(), secret.GetNamespace())

	usage := &apix.Usage{}
	assert.NoError(t, svc.GetDesiredKubeObject(usage, comp.GetName()+"-provider-conf-credentials-used-by-"+comp.GetName()+"-providerconfig"))
}

func Test_addUser(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	providerSecret := &xkube.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dummy",
		},
	}

	addUser(comp, svc, providerSecret, "unit")

	// then
	role := &pgv1alpha1.Role{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(role, fmt.Sprintf("%s-%s-role", comp.GetName(), "unit")))

	usage := &apix.Usage{}
	assert.NoError(t, svc.GetDesiredKubeObject(usage, "dummy-used-by-"+fmt.Sprintf("%s-%s-role", comp.GetName(), "unit")))

}

func Test_addDatabase(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	providerSecret := &xkube.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dummy",
		},
	}

	addDatabase(comp, svc, providerSecret, "unit")

	// then
	db := &pgv1alpha1.Database{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "unit")))

	usage := &apix.Usage{}
	assert.NoError(t, svc.GetDesiredKubeObject(usage, "dummy-used-by-"+fmt.Sprintf("%s-%s-database", comp.GetName(), "unit")))

}

func Test_addGrants(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	providerSecret := &xkube.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dummy",
		},
	}

	addGrants(comp, svc, providerSecret, "unit", "unit", []string{"ALL"})

	// then
	grant := &pgv1alpha1.Grant{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(grant, fmt.Sprintf("%s-%s-%s-grants", comp.GetName(), "unit", "unit")))

	usage := &apix.Usage{}
	assert.NoError(t, svc.GetDesiredKubeObject(usage, "dummy-used-by-"+fmt.Sprintf("%s-%s-%s-grants", comp.GetName(), "unit", "unit")))

}

func TestUserManagement(t *testing.T) {
	// given with empty accesss object
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/usermanagement/01-emptyaccess.yaml")

	// when applied
	assert.Nil(t, UserManagement(context.TODO(), svc))

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
	assert.Nil(t, UserManagement(context.TODO(), svc))

	// then expect database
	db = &pgv1alpha1.Database{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "prod")))

	// when adding user pointing to same db
	comp.Spec.Parameters.Service.Access = append(comp.Spec.Parameters.Service.Access, vshnv1.VSHNAccess{
		User:     ptr.To("another"),
		Database: ptr.To("prod"),
	})

	assert.NoError(t, svc.SetDesiredCompositeStatus(comp))
	assert.Nil(t, UserManagement(context.TODO(), svc))

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
