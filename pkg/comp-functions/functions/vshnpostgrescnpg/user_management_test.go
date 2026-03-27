package vshnpostgrescnpg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	cnpgUserMgmtFixture              = "vshn-postgres/usermanagement-cnpg/01_cnpg_usermgmt.yaml"
	cnpgUserMgmtPasswordReadyFixture = "vshn-postgres/usermanagement-cnpg/02_cnpg_usermgmt_password_ready.yaml"
)

func Test_addCnpgUser(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, cnpgUserMgmtFixture)
	comp := &vshnv1.VSHNPostgreSQL{}
	require.NoError(t, svc.GetObservedComposite(comp))

	secretName := addCnpgUser(comp, svc, "testuser")

	// Verify the KubeObject secret was created
	secret := &corev1.Secret{}
	require.NoError(t, svc.GetDesiredKubeObject(secret, secretName))

	// Secret should be in the instance namespace
	assert.Equal(t, comp.GetInstanceNamespace(), secret.Namespace)
	// Both "password" and "username" keys are required by CNPG's managed roles controller.
	// Missing "username" causes a silent reconcile failure (role is never created).
	assert.Contains(t, secret.StringData, "password")
	assert.Equal(t, "testuser", secret.StringData["username"])
}

func Test_addCnpgConnectionDetail_PasswordNotReady(t *testing.T) {
	// Fixture has no observed password secret — simulates first reconcile.
	svc := commontest.LoadRuntimeFromFile(t, cnpgUserMgmtFixture)
	comp := &vshnv1.VSHNPostgreSQL{}
	require.NoError(t, svc.GetObservedComposite(comp))

	secretName := addCnpgUser(comp, svc, "testuser")

	err := addCnpgConnectionDetail(comp, svc, secretName, "testuser", "testdb", nil)
	assert.NoError(t, err)

	// No user connection secret should be created yet
	userSecret := &corev1.Secret{}
	assert.Error(t, svc.GetDesiredKubeObject(userSecret, comp.GetName()+"-user-testuser"))
}

func Test_addCnpgConnectionDetail_PasswordReady(t *testing.T) {
	// Fixture has observed password secret with "testpassword".
	svc := commontest.LoadRuntimeFromFile(t, cnpgUserMgmtPasswordReadyFixture)
	comp := &vshnv1.VSHNPostgreSQL{}
	require.NoError(t, svc.GetObservedComposite(comp))

	secretName := addCnpgUser(comp, svc, "testuser")

	err := addCnpgConnectionDetail(comp, svc, secretName, "testuser", "testdb", nil)
	require.NoError(t, err)

	// Connection secret should be created in the claim namespace
	userSecret := &corev1.Secret{}
	require.NoError(t, svc.GetDesiredKubeObject(userSecret, comp.GetName()+"-user-testuser"))

	assert.Equal(t, "unit-test", userSecret.Namespace)
	assert.Equal(t, "pgsql-testuser", userSecret.Name)
	assert.Equal(t, []byte("testuser"), userSecret.Data["POSTGRESQL_USER"])
	assert.Equal(t, []byte("testpassword"), userSecret.Data["POSTGRESQL_PASSWORD"])
	assert.Equal(t, []byte("testdb"), userSecret.Data["POSTGRESQL_DB"])
	assert.Equal(t, []byte("localhost"), userSecret.Data["POSTGRESQL_HOST"])
	assert.Equal(t, []byte("5432"), userSecret.Data["POSTGRESQL_PORT"])
	assert.Equal(t, []byte("postgres://testuser:testpassword@localhost:5432/testdb"), userSecret.Data["POSTGRESQL_URL"])
}

func TestUserManagement_EmptyAccess(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, cnpgUserMgmtFixture)

	// Composite with no access entries
	result := UserManagement(context.TODO(), &vshnv1.VSHNPostgreSQL{}, svc)
	assert.Nil(t, result)
}

func TestUserManagement_SingleUser(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, cnpgUserMgmtFixture)
	comp := &vshnv1.VSHNPostgreSQL{}
	require.NoError(t, svc.GetObservedComposite(comp))

	result := UserManagement(context.TODO(), comp, svc)
	assert.Nil(t, result)

	// Password secret KubeObject should exist
	secretName := runtime.EscapeDNS1123(comp.GetName()+"-userpass-testuser", false)
	secret := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(secret, secretName))

	// Connection secret not yet created (password not in observed state)
	userSecret := &corev1.Secret{}
	assert.Error(t, svc.GetDesiredKubeObject(userSecret, comp.GetName()+"-user-testuser"))
}

func Test_addUserManagementValues_Roles(t *testing.T) {
	_, comp := getSvcCompCnpg(t)

	comp.Spec.Parameters.Service.Access = []vshnv1.VSHNAccess{
		{User: ptr.To("alice")},
		{User: ptr.To("bob"), Database: ptr.To("shared")},
	}

	values := map[string]any{
		"cluster": map[string]any{},
	}

	require.NoError(t, addUserManagementValues(comp, values))

	clusterMap := values["cluster"].(map[string]any)
	roles := clusterMap["roles"].([]map[string]any)
	require.Len(t, roles, 2)

	// Alice's role
	assert.Equal(t, "alice", roles[0]["name"])
	assert.Equal(t, true, roles[0]["login"])
	assert.Equal(t, "present", roles[0]["ensure"])
	aliceSecret := roles[0]["passwordSecret"].(map[string]any)
	assert.Equal(t, runtime.EscapeDNS1123(comp.GetName()+"-userpass-alice", false), aliceSecret["name"])

	// Bob's role
	assert.Equal(t, "bob", roles[1]["name"])
}

func Test_addUserManagementValues_Databases(t *testing.T) {
	_, comp := getSvcCompCnpg(t)

	// Two users pointing to the same database — should be deduplicated.
	comp.Spec.Parameters.Service.Access = []vshnv1.VSHNAccess{
		{User: ptr.To("alice"), Database: ptr.To("mydb")},
		{User: ptr.To("bob"), Database: ptr.To("mydb")},
	}

	values := map[string]any{
		"cluster": map[string]any{},
	}

	require.NoError(t, addUserManagementValues(comp, values))

	databases := values["databases"].([]map[string]any)
	// Only one database entry despite two users
	require.Len(t, databases, 1)
	assert.Equal(t, "mydb", databases[0]["name"])
	assert.Equal(t, "alice", databases[0]["owner"]) // first user wins
	assert.Equal(t, "present", databases[0]["ensure"])
	assert.Equal(t, "retain", databases[0]["databaseReclaimPolicy"])
}

func Test_addUserManagementValues_EmptyAccess(t *testing.T) {
	_, comp := getSvcCompCnpg(t)
	comp.Spec.Parameters.Service.Access = nil

	values := map[string]any{
		"cluster": map[string]any{},
	}

	require.NoError(t, addUserManagementValues(comp, values))

	// No roles or databases should be injected
	clusterMap := values["cluster"].(map[string]any)
	_, hasRoles := clusterMap["roles"]
	assert.False(t, hasRoles)
	_, hasDatabases := values["databases"]
	assert.False(t, hasDatabases)
}

func Test_buildPostgresURL(t *testing.T) {
	assert.Equal(t, "", buildPostgresURL("host", "user", "", "db"), "empty password should return empty string")
	assert.Equal(t,
		"postgres://alice:s3cr3t@myhost:5432/mydb",
		buildPostgresURL("myhost", "alice", "s3cr3t", "mydb"),
	)
}
