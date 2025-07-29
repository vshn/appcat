package maintenance

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRedis_DoMaintenance(t *testing.T) {
	tests := []struct {
		name                string
		instanceNamespace   string
		givenReleases       []client.Object
		expectedReleaseName string
		expectedErr         string
	}{
		{
			name:              "WhenMaintenance_ThenRelease",
			instanceNamespace: "test-namespace",
			givenReleases: []client.Object{
				&v1beta1.Release{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-release",
					},
					Spec: v1beta1.ReleaseSpec{
						ForProvider: v1beta1.ReleaseParameters{
							Namespace: "test-namespace",
							ValuesSpec: v1beta1.ValuesSpec{
								Values: runtime.RawExtension{
									Raw: []byte("{\"image\":{\"tag\":\"7.0\"}}"),
									Object: &runtime.Unknown{
										Raw: []byte("{\"image\":{\"tag\":\"7.0\"}}"),
									},
								},
							},
						},
					},
				},
				&v1beta1.Release{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-release-2",
					},
					Spec: v1beta1.ReleaseSpec{
						ForProvider: v1beta1.ReleaseParameters{
							Namespace: "test-namespace-2",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// GIVEN
			t.Setenv("INSTANCE_NAMESPACE", "test-namespace")
			t.Setenv("MAINTENANCE_URL", "https://hub.docker.com/v2/repositories/bitnamilegacy/redis/tags/?page_size=100")
			viper.AutomaticEnv()
			fClient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				WithObjects(tt.givenReleases...).
				Build()

			r := NewRedis(fClient, http.DefaultClient, &release.DefaultVersionHandler{}, logr.FromContextOrDiscard(context.Background()))
			ctx := context.Background()

			// WHEN
			err := r.DoMaintenance(ctx)

			// THEN
			assert.NoError(t, err)

			rel := &v1beta1.Release{}
			err = fClient.Get(ctx, types.NamespacedName{Name: "test-release"}, rel)
			assert.NoError(t, err)

			values := &map[string]interface{}{}
			err = json.Unmarshal(rel.Spec.ForProvider.Values.Raw, values)
			assert.NoError(t, err)

			tag, exists, err := unstructured.NestedFieldCopy(*values, "image", "tag")
			assert.NoError(t, err)
			assert.Equal(t, exists, true)

			imgTag, ok := tag.(string)
			assert.Equal(t, ok, true)

			assert.NotEqual(t, "7.0", imgTag)
		})
	}
}
