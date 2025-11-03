package vshnmariadb

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/crossplane/function-sdk-go/resource/composite"
	"github.com/stretchr/testify/assert"
	my1alpha1 "github.com/vshn/appcat/v4/apis/sql/mysql/v1alpha1"
	pgv1alpha1 "github.com/vshn/appcat/v4/apis/sql/postgresql/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func Test_addProviderConfig(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshnmariadb/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNMariaDB{}
	assert.NoError(t, svc.GetObservedComposite(comp))
	addProviderConfig(comp, svc)

	// then
	secret := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(secret, comp.GetName()+"-provider-conf-credentials"))

	config := &pgv1alpha1.ProviderConfig{}
	assert.NoError(t, svc.GetDesiredKubeObject(config, comp.GetName()+"-providerconfig"))

}

func Test_addUser(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshnmariadb/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNMariaDB{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	addUser(comp, svc, "unit")

	// then
	role := &pgv1alpha1.Role{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(role, fmt.Sprintf("%s-%s-role", comp.GetName(), "unit")))

}

func Test_addDatabase(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshnmariadb/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNMariaDB{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	addDatabase(comp, svc, "unit")

	// then
	db := &my1alpha1.Database{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "unit")))

}

func Test_addGrants(t *testing.T) {
	// given
	svc := commontest.LoadRuntimeFromFile(t, "vshnmariadb/usermanagement/01-emptyaccess.yaml")

	// when
	comp := &vshnv1.VSHNMariaDB{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	addGrants(comp, svc, "unit", "unit", []string{"ALL"})

	// then
	grant := &my1alpha1.Grant{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(grant, fmt.Sprintf("%s-%s-%s-grants", comp.GetName(), "unit", "unit")))

}

func TestUserManagement(t *testing.T) {
	// given with empty access object
	svc := commontest.LoadRuntimeFromFile(t, "vshnmariadb/usermanagement/01-emptyaccess.yaml")

	// when applied
	assert.Nil(t, UserManagement(context.TODO(), &vshnv1.VSHNMariaDB{}, svc))

	// then expect no database
	comp := &vshnv1.VSHNMariaDB{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	db := &my1alpha1.Database{}
	assert.Error(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "prod")))

	// when adding a user
	comp.Spec.Parameters.Service.Access = []vshnv1.VSHNAccess{
		{
			User: ptr.To("prod"),
		},
	}

	assert.NoError(t, setObservedComposition(svc, comp))

	assert.NoError(t, svc.SetDesiredCompositeStatus(comp))
	assert.Nil(t, UserManagement(context.TODO(), &vshnv1.VSHNMariaDB{}, svc))

	// then expect user to be created but not database yet (sequencing: user must be ready first)
	user := &my1alpha1.User{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, fmt.Sprintf("%s-%s-role", comp.GetName(), "prod")))

	// Database should not be created yet since user is not in observed state
	db = &my1alpha1.Database{}
	assert.Error(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "prod")))

	// when adding user pointing to same db
	comp.Spec.Parameters.Service.Access = append(comp.Spec.Parameters.Service.Access, vshnv1.VSHNAccess{
		User:     ptr.To("another"),
		Database: ptr.To("prod"),
	})

	assert.NoError(t, setObservedComposition(svc, comp))

	assert.NoError(t, svc.SetDesiredCompositeStatus(comp))
	assert.Nil(t, UserManagement(context.TODO(), &vshnv1.VSHNMariaDB{}, svc))

	// then expect both users but still no database (dependencies not ready)
	user = &my1alpha1.User{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, fmt.Sprintf("%s-%s-role", comp.GetName(), "prod")))
	user2 := &my1alpha1.User{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user2, fmt.Sprintf("%s-%s-role", comp.GetName(), "another")))

	// Database still not created since users are not in observed state
	db = &my1alpha1.Database{}
	assert.Error(t, svc.GetDesiredComposedResourceByName(db, fmt.Sprintf("%s-%s-database", comp.GetName(), "prod")))

	// When deleting user pointing to same db
	comp.Spec.Parameters.Service.Access = []vshnv1.VSHNAccess{
		{
			User:     ptr.To("another"),
			Database: ptr.To("prod"),
		},
	}

	assert.NoError(t, setObservedComposition(svc, comp))

	// then expect the other user to still be in desired
	user = &my1alpha1.User{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, fmt.Sprintf("%s-%s-role", comp.GetName(), "another")))
}

func setObservedComposition(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB) error {
	v := reflect.ValueOf(svc).Elem()
	val := v.FieldByName("observedComposite")
	val = reflect.NewAt(val.Type(), unsafe.Pointer(val.UnsafeAddr())).Elem()

	ccomp := composite.New()

	jcomp, err := json.Marshal(comp)
	if err != nil {
		return err
	}

	err = ccomp.Unstructured.UnmarshalJSON(jcomp)
	if err != nil {
		return err
	}

	compV := reflect.ValueOf(ccomp)
	val.Set(compV)

	return nil
}
