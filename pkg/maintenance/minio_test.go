package maintenance

import (
	"context"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	helmv1beta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_compareMinioVersions(t *testing.T) {
	type args struct {
		results    []helm.Result
		currentTag string
	}
	tests := []struct {
		name    string
		args    args
		wantTag string
		wantErr bool
	}{
		{
			name: "GivenEmpty_ThenSameTag",
			args: args{
				results:    []helm.Result{},
				currentTag: "RELEASE.2006-01-02T15-04-05Z",
			},
			wantTag: "RELEASE.2006-01-02T15-04-05Z",
		},
		{
			name: "GivenNonDateResults_ThenExpectSameTag",
			args: args{
				results: []helm.Result{
					getResult("Foo"),
				},
				currentTag: "RELEASE.2006-01-02T15-04-05Z",
			},
			wantTag: "RELEASE.2006-01-02T15-04-05Z",
		},
		{
			name: "GivenNewerResult_ThenExpectResult",
			args: args{
				results: []helm.Result{
					getResult("RELEASE.2007-01-02T15-04-05Z"),
				},
				currentTag: "RELEASE.2006-01-02T15-04-05Z",
			},
			wantTag: "RELEASE.2007-01-02T15-04-05Z",
		},
		{
			name: "GivenResultWithAppendix_ThenExpectWitouth",
			args: args{
				results: []helm.Result{
					getResult("RELEASE.2007-01-02T15-04-05Z"),
					getResult("RELEASE.2008-01-02T15-04-05Z.fips"),
				},
				currentTag: "RELEASE.2006-01-02T15-04-05Z",
			},
			wantTag: "RELEASE.2007-01-02T15-04-05Z",
		},
		{
			name: "GivenInvalidCurrentTag_ThenExpectValidTag",
			args: args{
				results: []helm.Result{
					getResult("RELEASE.2007-01-02T15-04-05Z"),
					getResult("RELEASE.2008-01-02T15-04-05Z.fips"),
				},
				currentTag: "Foo",
			},
			wantTag: minioTagPrefix + earliestTag,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, err := compareMinioVersions(tt.args.results, tt.args.currentTag)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantTag, tag)
		})
	}
}

func getResult(date string) helm.Result {
	return helm.Result{
		Name:        date,
		TagStatus:   helm.ActiveTagStatus,
		ContentType: helm.ImageContentType,
	}
}

func TestMinio_ensureTagIsNotNil(t *testing.T) {
	tests := []struct {
		name         string
		wantErr      bool
		givenRelease helmv1beta1.Release
		expect       string
	}{
		{
			name: "GivenReleasesNilTag_ThenExpectEmptyTag",
			givenRelease: helmv1beta1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrelease",
				},
				Spec: helmv1beta1.ReleaseSpec{
					ForProvider: helmv1beta1.ReleaseParameters{
						Namespace: "myns",
						ValuesSpec: v1beta1.ValuesSpec{
							Values: runtime.RawExtension{
								Raw: []byte(`{}`),
							},
						},
					},
				},
			},
			expect: `{"image":{"tag":""}}`,
		},
		{
			name: "GivenReleasesWrongNamespace_ThenExpectError",
			givenRelease: helmv1beta1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrelease",
				},
				Spec: helmv1beta1.ReleaseSpec{
					ForProvider: helmv1beta1.ReleaseParameters{
						ValuesSpec: v1beta1.ValuesSpec{
							Values: runtime.RawExtension{
								Raw: []byte(`{}`),
							},
						},
					},
				},
			},
			expect:  `{}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {

		ctx := context.TODO()
		assert.NoError(t, os.Setenv("INSTANCE_NAMESPACE", "myns"))
		viper.AutomaticEnv()

		fclient := fake.NewClientBuilder().
			WithScheme(pkg.SetupScheme()).
			WithObjects(&tt.givenRelease).
			Build()

		t.Run(tt.name, func(t *testing.T) {
			m := &Minio{
				k8sClient: fclient,
				log:       logr.Discard(),
			}
			if err := m.ensureTagIsNotNil(ctx, helm.NewValuePath("image", "tag")); (err != nil) != tt.wantErr {
				t.Errorf("Minio.ensureTagIsNotEmpty() error = %v, wantErr %v", err, tt.wantErr)
			}

			release := &helmv1beta1.Release{}
			assert.NoError(t, fclient.Get(ctx, client.ObjectKey{Name: tt.givenRelease.Name}, release))
			assert.Equal(t, []byte(tt.expect), release.Spec.ForProvider.Values.Raw)

		})
	}
}
