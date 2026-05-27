package webhooks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/compat"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func matrixConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: compat.MatrixConfigMapName, Namespace: compat.MatrixNamespace},
		Data: map[string]string{
			"currentRevision": "v3.73.0-v4.190.0",
			"matrix.yaml":     "postgresql:\n  - versionRange: \"14.x\"\n    compatibleRevisions: \"<v3.72.0\"\n",
		},
	}
}

func TestVersionCompat_denyIncompatible(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).WithObjects(matrixConfigMap()).Build()

	pg := &vshnv1.VSHNPostgreSQL{}
	pg.Spec.Parameters.Service.MajorVersion = "14" // incompatible with v3.73.0

	err := checkVersionCompat(context.Background(), c, pg)
	require.NotNil(t, err)
	assert.Contains(t, err.Detail, "incompatible")
}

func TestVersionCompat_allowCompatible(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).WithObjects(matrixConfigMap()).Build()

	pg := &vshnv1.VSHNPostgreSQL{}
	pg.Spec.Parameters.Service.MajorVersion = "16"

	assert.Nil(t, checkVersionCompat(context.Background(), c, pg))
}

func TestVersionCompat_missingMatrix_failOpen(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).Build() // no ConfigMap

	pg := &vshnv1.VSHNPostgreSQL{}
	pg.Spec.Parameters.Service.MajorVersion = "14"

	assert.Nil(t, checkVersionCompat(context.Background(), c, pg))
}

func TestVersionCompat_unknownType_failOpen(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).WithObjects(matrixConfigMap()).Build()
	assert.Nil(t, checkVersionCompat(context.Background(), c, &corev1.ConfigMap{}))
}

func TestVersionExtractor(t *testing.T) {
	pg := &vshnv1.VSHNPostgreSQL{}
	pg.Spec.Parameters.Service.MajorVersion = "16"
	svc, ver, ok := versionExtractor(pg)
	assert.True(t, ok)
	assert.Equal(t, "postgresql", svc)
	assert.Equal(t, "16", ver)

	fj := &vshnv1.VSHNForgejo{}
	fj.Spec.Parameters.Service.MajorVersion = "11"
	svc, ver, ok = versionExtractor(fj)
	assert.True(t, ok)
	assert.Equal(t, "forgejo", svc)
	assert.Equal(t, "11", ver)

	mdb := &vshnv1.VSHNMariaDB{}
	mdb.Spec.Parameters.Service.Version = "11.4"
	svc, _, ok = versionExtractor(mdb)
	assert.True(t, ok)
	assert.Equal(t, "mariadb", svc)

	_, _, ok = versionExtractor(&corev1.ConfigMap{})
	assert.False(t, ok)
}
