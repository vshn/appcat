package vshnmariadb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	pdbv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getComp() *vshnv1.VSHNMariaDB {
	return &vshnv1.VSHNMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myinstance",
		},
		Spec: vshnv1.VSHNMariaDBSpec{
			Parameters: vshnv1.VSHNMariaDBParameters{
				TLS: vshnv1.VSHNMariaDBTLSSpec{
					TLSEnabled: true,
				},
			},
		},
	}
}

func Test_copyCertificateSecret(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnmariadb/01-user-management.yaml")

	// Given TLS
	comp := getComp()

	// When applied
	assert.NoError(t, copyCertificateSecret(comp, svc, false))
	obj := &xkubev1.Object{}
	//Then expect secret
	assert.NoError(t, svc.GetDesiredComposedResourceByName(obj, comp.GetName()+"-proxysql-specific-certs"))

	svc = commontest.LoadRuntimeFromFile(t, "vshnmariadb/01-user-management.yaml")
	// Given no TLS
	comp.Spec.Parameters.TLS.TLSEnabled = false
	// When applied
	assert.NoError(t, copyCertificateSecret(comp, svc, false))
	//Then expect no secret
	assert.Error(t, svc.GetDesiredComposedResourceByName(obj, comp.GetName()+"-proxysql-specific-certs"))
}

func Test_createProxySQLConfig(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "empty.yaml")
	comp := getComp()

	expectedHash := "1bf00f243388da2aa7ba6b8d14bf7ad3"

	hash, err := createProxySQLConfig(comp, svc, false)
	assert.NoError(t, err)
	assert.Equal(t, expectedHash, hash)

	comp.Spec.Parameters.TLS.TLSEnabled = false

	hash, err = createProxySQLConfig(comp, svc, false)
	assert.NoError(t, err)
	assert.NotEqual(t, expectedHash, hash)
}

func Test_createProxySQLHeadlessService(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "empty.yaml")
	comp := getComp()

	assert.NoError(t, createProxySQLHeadlessService(comp, svc, false))

	service := &corev1.Service{}
	assert.NoError(t, svc.GetDesiredKubeObject(service, comp.GetName()+"-proxysql-headless-service"))
}

func Test_createProxySQLStatefulset(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "empty.yaml")
	comp := getComp()

	// given TLS enabled
	assert.NoError(t, createProxySQLStatefulset(comp, svc, "1", false))

	sts := &appsv1.StatefulSet{}
	assert.NoError(t, svc.GetDesiredKubeObject(sts, comp.GetName()+"-proxysql-sts"))

	// Then expect cert mounts
	assert.Len(t, sts.Spec.Template.Spec.Containers[0].VolumeMounts, 5)
	assert.Len(t, sts.Spec.Template.Spec.Volumes, 3)

	comp.Spec.Parameters.TLS.TLSEnabled = false

	// given TLS disabled
	assert.NoError(t, createProxySQLStatefulset(comp, svc, "1", false))

	sts = &appsv1.StatefulSet{}
	assert.NoError(t, svc.GetDesiredKubeObject(sts, comp.GetName()+"-proxysql-sts"))

	// Then expect no cert mounts
	assert.Len(t, sts.Spec.Template.Spec.Containers[0].VolumeMounts, 2)
	assert.Len(t, sts.Spec.Template.Spec.Volumes, 2)
}

func Test_createProxySQLPDB(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "empty.yaml")
	comp := getComp()

	assert.NoError(t, createProxySQLPDB(comp, svc, false))

	pdb := &pdbv1.PodDisruptionBudget{}

	assert.NoError(t, svc.GetDesiredKubeObject(pdb, comp.GetName()+"-proxysql-pdb"))
}
