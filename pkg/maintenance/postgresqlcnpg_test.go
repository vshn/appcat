// go:build ignore

package maintenance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	"github.com/vshn/appcat/v4/pkg"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_RetrieveObjectsAndPatchImageCatalog(t *testing.T) {
	const (
		repo              = "cloudnative-pg/postgresql"
		instanceNamespace = "test"
		icName            = "imagecatalog-cluster"
		compName          = "my-comp"
	)

	fakeCluster := &cnpgv1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      "postgresql",
			Namespace: instanceNamespace,
		},
		Spec: cnpgv1.ClusterSpec{
			ImageCatalogRef: &cnpgv1.ClusterSpecImageCatalogRef{
				Major: 16,
				Name:  icName,
			},
		},
	}

	fakeImageCatalog := &cnpgv1.ImageCatalog{
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
			fakeCluster,
			fakeImageCatalog,
		}...).
		Build()

	httpImageCatalog := cnpgv1.ImageCatalog{
		ObjectMeta: v1.ObjectMeta{
			Name:      icName,
			Namespace: instanceNamespace,
		},
		Spec: cnpgv1.ImageCatalogSpec{
			Images: []cnpgv1.ImageCatalogSpecImagesItem{
				{
					Image: repo + ":17.7",
					Major: 17,
				},
				{
					Image: repo + ":16.7",
					Major: 16,
				},
				{
					Image: repo + ":15.7",
					Major: 15,
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		imageJson, err := json.Marshal(httpImageCatalog)
		assert.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		w.Write(imageJson)
	}))
	defer server.Close()

	p := PostgreSQLCNPG{
		k8sClient:         fclient,
		httpClient:        http.DefaultClient,
		instanceNamespace: instanceNamespace,
		compName:          compName,
		catalogURL:        server.URL,
	}

	latestCatalog, err := p.getLatestImageCatalog(context.TODO(), p.catalogURL)
	assert.NoError(t, err)

	assert.Equal(t, httpImageCatalog.Spec, latestCatalog.Spec)

	err = p.applyImageCatalog(context.TODO(), latestCatalog, fakeCluster)
	assert.NoError(t, err)

}
