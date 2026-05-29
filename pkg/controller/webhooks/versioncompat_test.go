package webhooks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/compat"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func matrixConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: compat.MatrixConfigMapName, Namespace: compat.MatrixNamespace},
		Data: map[string]string{
			"matrix.yaml": "postgresql:\n  - versionRange: \"14.x\"\n    compatibleRevisions: \"<v3.72.0\"\n",
		},
	}
}

// pgClaim builds an unstructured VSHNPostgreSQL carrying a pinned revision in its
// compositionRevisionSelector, as release management would set it.
func pgClaim(name, ns, revision string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "vshn.appcat.vshn.io",
		Version: "v1",
		Kind:    "VSHNPostgreSQL",
	})
	u.SetName(name)
	u.SetNamespace(ns)
	if revision != "" {
		_ = unstructured.SetNestedStringMap(u.Object,
			map[string]string{release.RevisionLabel: revision},
			"spec", "compositionRevisionSelector", "matchLabels")
	}
	return u
}

// The pinned-revision deny/allow decision cannot be exercised through the fake
// client: it round-trips objects through the typed VSHNPostgreSQL scheme, which
// does not model spec.compositionRevisionSelector, so the selector is dropped.
// (The real cached client reads the API server, where the CRD schema keeps the
// Crossplane composition fields.) So the two pure halves are tested directly:
// revisionFromUnstructured (extraction) and verdictFieldError (decision).

func TestRevisionFromUnstructured(t *testing.T) {
	assert.Equal(t, "v3.73.0-v4.190.0",
		revisionFromUnstructured(pgClaim("inst", "ns", "v3.73.0-v4.190.0")))
	assert.Equal(t, "", revisionFromUnstructured(pgClaim("inst", "ns", "")))
}

func TestVerdictFieldError(t *testing.T) {
	m, err := compat.ParseMatrix([]byte("postgresql:\n  - versionRange: \"14.x\"\n    compatibleRevisions: \"<v3.72.0\"\n"))
	require.NoError(t, err)

	// pg 14 on v3.73.0 violates "<v3.72.0" -> deny
	fe := verdictFieldError(m, "v3.73.0-v4.190.0", "postgresql", "14")
	require.NotNil(t, fe)
	assert.Contains(t, fe.Detail, "incompatible")

	// pg 14 on v3.71.0 satisfies "<v3.72.0" -> allow
	assert.Nil(t, verdictFieldError(m, "v3.71.0-v4.180.0", "postgresql", "14"))
}

func TestVersionCompat_unpinned_failOpen(t *testing.T) {
	// no selector on the live object -> empty revision -> fail open
	c := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).
		WithObjects(matrixConfigMap(), pgClaim("inst", "ns", "")).Build()

	pg := &vshnv1.VSHNPostgreSQL{}
	pg.SetName("inst")
	pg.SetNamespace("ns")
	pg.Spec.Parameters.Service.MajorVersion = "14"

	assert.Nil(t, checkVersionCompat(context.Background(), c, pg))
}

func TestVersionCompat_missingMatrix_failOpen(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).
		WithObjects(pgClaim("inst", "ns", "v3.73.0-v4.190.0")).Build() // no matrix CM

	pg := &vshnv1.VSHNPostgreSQL{}
	pg.SetName("inst")
	pg.SetNamespace("ns")
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

	mdb := &vshnv1.VSHNMariaDB{}
	mdb.Spec.Parameters.Service.Version = "11.4"
	svc, _, ok = versionExtractor(mdb)
	assert.True(t, ok)
	assert.Equal(t, "mariadb", svc)

	_, _, ok = versionExtractor(&corev1.ConfigMap{})
	assert.False(t, ok)
}
