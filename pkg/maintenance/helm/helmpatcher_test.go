package helm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_PatchRelease(t *testing.T) {
	tests := []struct {
		name              string
		instanceNamespace string
		version           semver.Version
		release           *v1beta1.Release
		existingReleases  []client.Object
		expectedValues    runtime.RawExtension
		expectedErr       string
	}{
		{
			name:              "WhenPatch_ThenReleaseNewVersion",
			instanceNamespace: "test-namespace",
			version:           semver.MustParse("7.2.4"),
			release: &v1beta1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
				Spec: v1beta1.ReleaseSpec{
					ForProvider: v1beta1.ReleaseParameters{
						ValuesSpec: v1beta1.ValuesSpec{
							Values: runtime.RawExtension{
								Raw: []byte("{\"image\":{\"tag\":\"7.0\"}}"),
							},
						},
					},
				},
			},
			existingReleases: []client.Object{
				&v1beta1.Release{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-release",
					},
					Spec: v1beta1.ReleaseSpec{
						ForProvider: v1beta1.ReleaseParameters{
							ValuesSpec: v1beta1.ValuesSpec{
								Values: runtime.RawExtension{
									Raw: []byte("{\"image\":{\"tag\":\"7.0\"}}"),
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
							ValuesSpec: v1beta1.ValuesSpec{
								Values: runtime.RawExtension{
									Raw: []byte("{\"image\":{\"tag\":\"7.1\"}}"),
								},
							},
						},
					},
				},
			},
			expectedValues: runtime.RawExtension{
				Raw: []byte("{\"image\":{\"tag\":\"7.2.4\"}}"),
			},
		},
		{
			name:              "WhenPatchAndNoRelease_ThenError",
			instanceNamespace: "test-namespace",
			version:           semver.MustParse("7.2.4"),
			release: &v1beta1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
				Spec: v1beta1.ReleaseSpec{
					ForProvider: v1beta1.ReleaseParameters{
						ValuesSpec: v1beta1.ValuesSpec{
							Values: runtime.RawExtension{
								Raw: []byte("{\"image\":{\"tag\":\"7.0\"}}"),
							},
						},
					},
				},
			},
			expectedErr: "releases.helm.crossplane.io \"test-release\" not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// GIVEN
			fClient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				WithObjects(tt.existingReleases...).
				Build()
			r := ImagePatcher{
				k8sClient:         fClient,
				log:               logr.Logger{},
				instanceNamespace: tt.instanceNamespace,
			}
			ctx := context.Background()

			// WHEN
			err := r.patchRelease(ctx, tt.version.String(), tt.release, "/spec/forProvider/values/image/tag")

			// THEN
			if tt.expectedErr != "" {
				assert.Equal(t, err.Error(), tt.expectedErr)
				return
			}
			assert.NoError(t, err)
			rel := &v1beta1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
			}
			err = fClient.Get(ctx, types.NamespacedName{Name: "test-release"}, rel)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValues, rel.Spec.ForProvider.Values)
		})
	}
}

func Test_compareSemanticVersion(t *testing.T) {
	defaultV, _ := semver.ParseTolerant("7.0")
	tests := []struct {
		name              string
		instanceNamespace string
		release           *v1beta1.Release
		results           []Result
		expectedVer       semver.Version
		expectedNew       bool
		expectedErr       string
	}{
		{
			name:              "WhenNewVersionFromReleases_ThenGetNewVersion",
			instanceNamespace: "test-namespace",
			release: &v1beta1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
				Spec: v1beta1.ReleaseSpec{
					ForProvider: v1beta1.ReleaseParameters{
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
			results: []Result{
				getResult("6.5.4", "active", "image"),
				getResult("6.6.4", "inactive", "image"),
				getResult("6.8", "active", "image"),
				getResult("7.0", "active", "image"),
				getResult("7.0.1", "active", "registry"),
				getResult("7.0.4", "active", "image"),
				getResult("7.1.4", "active", "image"),
				getResult("7.2.4", "inactive", "image"),
				getResult("7.1.12-alpha", "active", "image"),
				getResult("7.3.11", "active", "image"),
				getResult("7.0.13", "active", "image"),
				getResult("7.0.13-debian", "active", "image"),
				getResult("7.0.14-alpha", "active", "image"),
				getResult("7.0.14%alpa", "active", "image"),
			},
			expectedVer: semver.MustParse("7.0.13"),
		},
		{
			name:              "WhenNoNewVersion_ThenReleaseNoNewVersion",
			instanceNamespace: "test-namespace",
			release: &v1beta1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
				Spec: v1beta1.ReleaseSpec{
					ForProvider: v1beta1.ReleaseParameters{
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
			results: []Result{
				getResult("6.5.4", "active", "image"),
				getResult("6.6.4", "inactive", "image"),
				getResult("6.8", "active", "image"),
				getResult("7.0", "active", "image"),
				getResult("7.0.1", "active", "registry"),
				getResult("7.1.4", "active", "image"),
				getResult("7.2.4", "inactive", "image"),
				getResult("7.1.12-alpha", "active", "image"),
				getResult("7.3.11", "active", "image"),
				getResult("7.0.14-alpha", "active", "image"),
				getResult("7.0.14%alpa", "active", "image"),
			},
			expectedVer: defaultV,
		},
		{
			name:              "WhenNoResult_ThenReleaseNoNewVersion",
			instanceNamespace: "test-namespace",
			release: &v1beta1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
				Spec: v1beta1.ReleaseSpec{
					ForProvider: v1beta1.ReleaseParameters{
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
			results:     []Result{},
			expectedVer: defaultV,
		},
		{
			name:              "WhenCurrentVersionWrong_ThenError",
			instanceNamespace: "test-namespace",
			release: &v1beta1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
				Spec: v1beta1.ReleaseSpec{
					ForProvider: v1beta1.ReleaseParameters{
						ValuesSpec: v1beta1.ValuesSpec{
							Values: runtime.RawExtension{
								Raw: []byte("{\"image\":{\"tag\":\"miss\"}}"),
								Object: &runtime.Unknown{
									Raw: []byte("{\"image\":{\"tag\":\"miss\"}}"),
								},
							},
						},
					},
				},
			},
			results:     []Result{},
			expectedErr: "current version miss of release is not sem ver: Invalid character(s) found in major number \"0miss\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// GIVEN
			r := ImagePatcher{
				log:               logr.Logger{},
				instanceNamespace: tt.instanceNamespace,
			}

			// WHEN
			tag, err := r.getCurrentTagFromRelease(tt.release, []string{"image", "tag"})
			assert.NoError(t, err)
			version, err := SemVerPatchesOnly(tt.results, tag)

			// THEN
			if tt.expectedErr != "" {
				assert.Equal(t, err.Error(), tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedVer.String(), version)
		})
	}
}

func Test_GetRelease(t *testing.T) {
	tests := []struct {
		name                string
		instanceNamespace   string
		givenReleases       []client.Object
		expectedReleaseName string
		expectedErr         string
	}{
		{
			name:              "WhenRelease_ThenReturnRelease",
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
			expectedReleaseName: "test-release",
		},
		{
			name:              "WhenNoRelease_ThenReturnError",
			instanceNamespace: "test-namespace",
			givenReleases: []client.Object{
				&v1beta1.Release{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-release",
					},
					Spec: v1beta1.ReleaseSpec{
						ForProvider: v1beta1.ReleaseParameters{
							Namespace: "test-namespace-1",
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
			expectedErr: "cannot find release in namespace test-namespace",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// GIVEN
			fClient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				WithObjects(tt.givenReleases...).
				Build()
			ctx := context.Background()

			// WHEN
			release, err := GetRelease(ctx, fClient, tt.instanceNamespace)

			// THEN
			if tt.expectedErr != "" {
				assert.Equal(t, err.Error(), tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedReleaseName, release.Name)
		})
	}
}

func Test_GetVersions(t *testing.T) {
	tests := []struct {
		name              string
		instanceNamespace string
		server            *httptest.Server
		expectedResults   []Result
	}{
		{
			name:              "WhenRelease_ThenReturnRelease",
			instanceNamespace: "test-namespace",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				p := Payload{
					Results: []Result{
						getResult("1.0.0", "active", "image"),
						getResult("2.0.0", "inactive", "image"),
					},
				}
				payload, err := json.Marshal(p)
				assert.NoError(t, err)

				_, err = w.Write(payload)
				assert.NoError(t, err)
			})),
			expectedResults: []Result{
				getResult("1.0.0", "active", "image"),
				getResult("2.0.0", "inactive", "image"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// GIVEN
			r := ImagePatcher{
				httpClient:        tt.server.Client(),
				log:               logr.Logger{},
				instanceNamespace: tt.instanceNamespace,
			}

			defer tt.server.Close()

			// WHEN
			res, err := r.getVersions(tt.server.URL + "?size=100&")

			// THEN
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResults, res)
		})
	}
}

func getResult(n, s, ct string) Result {
	return Result{
		Name:        n,
		TagStatus:   s,
		ContentType: ct,
	}
}
