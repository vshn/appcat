package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testRepo              = "cloudnative-pg/postgresql"
	testInstanceNamespace = "vshn-postgresql-test"
	testIcName            = "imagecatalog-cluster"
	testCompName          = "my-comp"
	testClaimName         = "my-claim"
	testClaimNamespace    = "my-namespace"
)

// newTestImageCatalogServer returns an httptest.Server that serves the given ImageCatalog as JSON.
func newTestImageCatalogServer(t *testing.T, catalog cnpgv1.ImageCatalog) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := json.Marshal(catalog)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	}))
}

func newTestObjects(majorVersion int) (
	comp *vshnv1.XVSHNPostgreSQL,
	cluster *cnpgv1.Cluster,
	imageCatalog *cnpgv1.ImageCatalog,
) {
	comp = &vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: testCompName,
		},
		Spec: vshnv1.XVSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Service: vshnv1.VSHNPostgreSQLServiceSpec{
					MajorVersion: fmt.Sprintf("%d", majorVersion),
				},
			},
		},
	}

	cluster = &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgresql",
			Namespace: testInstanceNamespace,
		},
		Spec: cnpgv1.ClusterSpec{
			Instances: 1,
			ImageCatalogRef: &cnpgv1.ClusterSpecImageCatalogRef{
				Major: majorVersion,
				Name:  testIcName,
			},
		},
		Status: cnpgv1.ClusterStatus{
			ReadyInstances: 1,
			Image:          testRepo + ":16.1",
		},
	}

	imageCatalog = &cnpgv1.ImageCatalog{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testIcName,
			Namespace: testInstanceNamespace,
		},
		Spec: cnpgv1.ImageCatalogSpec{
			Images: []cnpgv1.ImageCatalogSpecImagesItem{
				{Image: testRepo + ":17.1", Major: 17},
				{Image: testRepo + ":16.1", Major: 16},
				{Image: testRepo + ":15.1", Major: 15},
			},
		},
	}

	return
}

func newTestRunner(fclient client.WithWatch, catalogURL string) *PostgreSQLCNPG {
	return &PostgreSQLCNPG{
		k8sClient:         fclient,
		httpClient:        http.DefaultClient,
		instanceNamespace: testInstanceNamespace,
		compName:          testCompName,
		claimName:         testClaimName,
		claimNamespace:    testClaimNamespace,
		catalogURL:        catalogURL,
	}
}

func Test_DoMaintenance_Standalone_UpdatesImageCatalog(t *testing.T) {
	comp, cluster, imageCatalog := newTestObjects(16)

	claim := &vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClaimName,
			Namespace: testClaimNamespace,
		},
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Service: vshnv1.VSHNPostgreSQLServiceSpec{
					MajorVersion: "16",
				},
			},
		},
	}

	latestCatalog := cnpgv1.ImageCatalog{
		Spec: cnpgv1.ImageCatalogSpec{
			Images: []cnpgv1.ImageCatalogSpecImagesItem{
				{Image: testRepo + ":17.7", Major: 17},
				{Image: testRepo + ":16.7", Major: 16},
				{Image: testRepo + ":15.7", Major: 15},
			},
		},
	}

	server := newTestImageCatalogServer(t, latestCatalog)
	defer server.Close()

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(comp, cluster, imageCatalog, claim).
		WithStatusSubresource(&vshnv1.VSHNPostgreSQL{}).
		Build()

	p := newTestRunner(fclient, server.URL)

	require.NoError(t, p.DoMaintenance(context.TODO()))

	// Image catalog should have been updated to the latest images.
	updated := &cnpgv1.ImageCatalog{}
	require.NoError(t, fclient.Get(context.TODO(), client.ObjectKey{Name: testIcName, Namespace: testInstanceNamespace}, updated))
	assert.Equal(t, latestCatalog.Spec.Images, updated.Spec.Images)

	// Version 16 is still in the catalog, so EOL should NOT be set on the claim.
	updatedClaim := &vshnv1.VSHNPostgreSQL{}
	require.NoError(t, fclient.Get(context.TODO(), client.ObjectKey{Name: testClaimName, Namespace: testClaimNamespace}, updatedClaim))
	assert.False(t, updatedClaim.Status.IsEOL)
}

func Test_DoMaintenance_Standalone_EOLVersion_SetsEOLOnClaim(t *testing.T) {
	// Major version 14 — not in the catalog, so it's EOL.
	comp, cluster, imageCatalog := newTestObjects(14)

	claim := &vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClaimName,
			Namespace: testClaimNamespace,
		},
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Service: vshnv1.VSHNPostgreSQLServiceSpec{
					MajorVersion: "14",
				},
			},
		},
	}

	// Catalog does not include major 14.
	latestCatalog := cnpgv1.ImageCatalog{
		Spec: cnpgv1.ImageCatalogSpec{
			Images: []cnpgv1.ImageCatalogSpecImagesItem{
				{Image: testRepo + ":17.7", Major: 17},
				{Image: testRepo + ":16.7", Major: 16},
				{Image: testRepo + ":15.7", Major: 15},
			},
		},
	}

	server := newTestImageCatalogServer(t, latestCatalog)
	defer server.Close()

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(comp, cluster, imageCatalog, claim).
		WithStatusSubresource(&vshnv1.VSHNPostgreSQL{}).
		Build()

	p := newTestRunner(fclient, server.URL)

	require.NoError(t, p.DoMaintenance(context.TODO()))

	// EOL should be set on the claim.
	updatedClaim := &vshnv1.VSHNPostgreSQL{}
	require.NoError(t, fclient.Get(context.TODO(), client.ObjectKey{Name: testClaimName, Namespace: testClaimNamespace}, updatedClaim))
	assert.True(t, updatedClaim.Status.IsEOL)
}

func Test_DoMaintenance_Nested_NoClaim_SucceedsAndSkipsEOL(t *testing.T) {
	comp, cluster, imageCatalog := newTestObjects(16)

	latestCatalog := cnpgv1.ImageCatalog{
		Spec: cnpgv1.ImageCatalogSpec{
			Images: []cnpgv1.ImageCatalogSpecImagesItem{
				{Image: testRepo + ":17.7", Major: 17},
				{Image: testRepo + ":16.7", Major: 16},
				{Image: testRepo + ":15.7", Major: 15},
			},
		},
	}

	server := newTestImageCatalogServer(t, latestCatalog)
	defer server.Close()

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(comp, cluster, imageCatalog).
		// No claim added.
		Build()

	p := newTestRunner(fclient, server.URL)

	// Should succeed even without a VSHNPostgreSQL claim.
	require.NoError(t, p.DoMaintenance(context.TODO()))

	// Image catalog should still have been updated.
	updated := &cnpgv1.ImageCatalog{}
	require.NoError(t, fclient.Get(context.TODO(), client.ObjectKey{Name: testIcName, Namespace: testInstanceNamespace}, updated))
	assert.Equal(t, latestCatalog.Spec.Images, updated.Spec.Images)
}

func Test_RetrieveObjectsAndPatchImageCatalog(t *testing.T) {
	_, fakeCluster, fakeImageCatalog := newTestObjects(16)

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(fakeCluster, fakeImageCatalog).
		Build()

	httpImageCatalog := cnpgv1.ImageCatalog{
		Spec: cnpgv1.ImageCatalogSpec{
			Images: []cnpgv1.ImageCatalogSpecImagesItem{
				{Image: testRepo + ":17.7", Major: 17},
				{Image: testRepo + ":16.7", Major: 16},
				{Image: testRepo + ":15.7", Major: 15},
			},
		},
	}

	server := newTestImageCatalogServer(t, httpImageCatalog)
	defer server.Close()

	p := &PostgreSQLCNPG{
		k8sClient:         fclient,
		httpClient:        http.DefaultClient,
		instanceNamespace: testInstanceNamespace,
		compName:          testCompName,
		catalogURL:        server.URL,
	}

	latestCatalog, err := p.getLatestImageCatalog(context.TODO(), p.catalogURL)
	require.NoError(t, err)
	assert.Equal(t, httpImageCatalog.Spec, latestCatalog.Spec)

	err = p.applyImageCatalog(context.TODO(), latestCatalog, fakeCluster)
	require.NoError(t, err)
}
