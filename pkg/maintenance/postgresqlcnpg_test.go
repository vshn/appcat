// go:build ignore

package maintenance

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_ParseGhcrLinkHeader(t *testing.T) {
	// Normal link header
	linkHeader := getMockLinkHeader("/v2/cloudnative-pg/postgresql/tags/list?last=13.16&n=1000", "next")
	found, nextURL := parseGhcrLinkHeader(linkHeader)
	assert.True(t, found)
	assert.Equal(t, "https://ghcr.io/v2/cloudnative-pg/postgresql/tags/list?last=13.16&n=1000", nextURL)

	// Normal link header but domain is provided
	linkHeader = getMockLinkHeader("ghcr.io/v2/cloudnative-pg/postgresql/tags/list?last=13.16&n=1000", "next")
	found, nextURL = parseGhcrLinkHeader(linkHeader)
	assert.True(t, found)
	assert.Equal(t, "https://ghcr.io/v2/cloudnative-pg/postgresql/tags/list?last=13.16&n=1000", nextURL)

	// No link header
	linkHeader.Del("Link")
	found, nextURL = parseGhcrLinkHeader(linkHeader)
	assert.False(t, found)
	assert.Empty(t, nextURL)

	// Link header with unhandled rel
	linkHeader = getMockLinkHeader("/v2/cloudnative-pg/postgresql/tags/list?last=13.16&n=1000", "prev")
	found, nextURL = parseGhcrLinkHeader(linkHeader)
	assert.False(t, found)
	assert.Empty(t, nextURL)
}

func Test_GetPsqlTags(t *testing.T) {
	p := PostgreSQLCNPG{
		httpClient: http.DefaultClient,
	}

	v, err := p.getRegistryVersions(apiUrl)
	assert.NoError(t, err)
	assert.NotEmpty(t, v)
	assert.NotEmpty(t, v.GetVersions())
}

func Test_RetrieveObjectsAndPatchImageCatalog(t *testing.T) {
	const (
		repo              = "cloudnative-pg/postgresql"
		instanceNamespace = "test"
		icName            = "imagecatalog-cluster"
		compName          = "my-comp"
	)

	fakeCluster := cnpgv1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      compName + "-cluster",
			Namespace: instanceNamespace,
		},
		Spec: cnpgv1.ClusterSpec{
			ImageCatalogRef: &cnpgv1.ClusterSpecImageCatalogRef{
				Major: 16,
				Name:  icName,
			},
		},
	}

	fakeImageCatalog := cnpgv1.ImageCatalog{
		ObjectMeta: v1.ObjectMeta{
			Name:      icName,
			Namespace: instanceNamespace,
		},
		Spec: cnpgv1.ImageCatalogSpec{
			Images: []cnpgv1.ImageCatalogSpecImagesItem{
				{
					Image: repo + ":17.1",
					Major: 17,
				},
				{
					Image: repo + ":16.1",
					Major: 16,
				},
				{
					Image: repo + ":15.1",
					Major: 15,
				},
			},
		},
	}

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects([]client.Object{
			&fakeCluster,
			&fakeImageCatalog,
		}...).
		Build()

	p := PostgreSQLCNPG{
		k8sClient:         fclient,
		httpClient:        http.DefaultClient,
		instanceNamespace: instanceNamespace,
		compName:          compName,
	}

	registryResult := &helm.RegistryResult{
		Name: repo,
		Tags: []string{
			"17.0", "17.1", "17.2", "17.3",
			"16.1", "16.2", "16.5",
			"15.1", "15.6", "15.9",
		},
	}

	instanceCluster, err := p.getCompositeCluster(context.TODO())
	assert.NoError(t, err)
	assert.Equal(t, fakeCluster.Name, instanceCluster.Name)

	imageCatalogNameFromInstanceCluster := instanceCluster.Spec.ImageCatalogRef.Name
	imageCatalog, err := p.getImageCatalog(context.TODO(), imageCatalogNameFromInstanceCluster)
	assert.NoError(t, err)
	assert.Equal(t, icName, imageCatalog.Name)

	updated, err := p.setLatestVersionInCatalog(imageCatalog, registryResult)
	assert.NoError(t, err)
	assert.True(t, updated)

	for i, image := range imageCatalog.Spec.Images {
		t.Logf("Checking imageCatalog.Spec.Images[%d]: %d - %s", i, image.Major, image.Image)
		thisImageTag := strings.Split(image.Image, ":")[1]
		assert.NotEmpty(t, thisImageTag)

		switch image.Major {
		case 17:
			assert.Equal(t, "17.3", thisImageTag)
		case 16:
			assert.Equal(t, "16.5", thisImageTag)
		case 15:
			assert.Equal(t, "15.9", thisImageTag)
		default:
			t.Fatalf("Unhandled major: %d", image.Major)
		}
	}
}

func getMockLinkHeader(ref string, rel string) http.Header {
	hd := http.Header{}
	hd.Add("Link", fmt.Sprintf(`<%s>; rel="%s"`, ref, rel))
	return hd
}
