package vshnmariadb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func TestMariadbDeploy(t *testing.T) {

	svc, comp := getMariadbComp(t)

	ctx := context.TODO()

	rootUser := "root"
	rootPassword := "mariadb123"
	mariadbHost := "mariadb-gc9x4.vshn-mariadb-mariadb-gc9x4.svc.cluster.local"
	mariadbPort := "3306"
	mariadbUrl := "mysql://mariadb-gc9x4.vshn-mariadb-mariadb-gc9x4.svc.cluster.local:3306"

	assert.Nil(t, DeployMariadb(ctx, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, comp.Name+"-ns"))
	assert.Equal(t, string("vshn"), ns.GetLabels()[utils.OrgLabelName])

	r := &xhelmbeta1.Release{}
	assert.NoError(t, svc.GetObservedComposedResource(r, comp.Name+"-release"))

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(roleBinding, comp.Name+"-services-read-rolebinding"))

	cd := svc.GetConnectionDetails()
	assert.Equal(t, mariadbHost, string(cd["MARIADB_HOST"]))
	assert.Equal(t, mariadbPort, string(cd["MARIADB_PORT"]))
	assert.Equal(t, mariadbUrl, string(cd["MARIADB_URL"]))
	assert.Equal(t, rootUser, string(cd["MARIADB_USER"]))
	assert.Equal(t, rootPassword, string(cd["MARIADB_PASSWORD"]))

}

func getMariadbComp(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNMariaDB) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnmariadb/deploy/01_default.yaml")

	comp := &vshnv1.VSHNMariaDB{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
