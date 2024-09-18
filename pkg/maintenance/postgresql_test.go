package maintenance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPostgreSQL_getLatestMinorversion(t *testing.T) {
	type args struct {
		vers        string
		versionList *pgVersions
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "GivenMostCurrenMinorVersion_ThenExpectSameVersion",
			args: args{
				vers: "15.1",
				versionList: &pgVersions{
					Postgresql: []string{
						"16.1",
						"16.0",
						"15.1",
						"15.0",
						"14.4",
					},
				},
			},
			want:    "15.1",
			wantErr: false,
		},
		{
			name: "GivenNewerMinorAvailable_ThenExpectNewVersion",
			args: args{
				vers: "15.0",
				versionList: &pgVersions{
					Postgresql: []string{
						"16.1",
						"16.0",
						"15.1",
						"15.0",
						"14.4",
					},
				},
			},
			want: "15.1",
		},
		{
			name: "GivenNoVersionList_ThenExpectSameVersion",
			args: args{
				vers: "15.1",
			},
			want: "15.1",
		},
		{
			name: "GivenNewerVersionTanAvailable_ThenExpectSameVersion",
			args: args{
				vers: "15.1",
				versionList: &pgVersions{
					Postgresql: []string{
						"15.0",
						"14.5",
					},
				},
			},
			want: "15.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PostgreSQL{
				log: logr.Discard(),
				ctx: context.TODO(),
			}
			got, err := p.getLatestMinorversion(tt.args.vers, tt.args.versionList)
			if (err != nil) != tt.wantErr {
				t.Errorf("PostgreSQL.getLatestMinorversion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("PostgreSQL.getLatestMinorversion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPostgreSQL_fetchVersionList(t *testing.T) {
	tests := []struct {
		name    string
		want    *pgVersions
		wantErr bool
	}{
		{
			name: "GivenWorkingServer_ThenExpectListOfVersions",
			want: &pgVersions{
				Postgresql: []string{
					"15.1",
					"15.0",
					"14.5",
					"14.4",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			server := getVersionTestHTTPServer(t)
			defer server.Close()

			p := &PostgreSQL{
				log: logr.Discard(),
				ctx: context.TODO(),
			}
			got, err := p.fetchVersionList(server.URL)
			if (err != nil) != tt.wantErr {
				t.Errorf("PostgreSQL.fetchVersionList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PostgreSQL.fetchVersionList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func getVersionTestHTTPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		byteResp := []byte{}

		path := r.URL.Path

		if strings.Contains(path, "version") {
			versions := pgVersions{
				Postgresql: []string{
					"15.1",
					"15.0",
					"14.5",
					"14.4",
				},
			}

			bytes, err := json.Marshal(versions)
			assert.NoError(t, err)
			byteResp = bytes
		}

		if strings.Contains(path, "login") {
			token := authToken{
				AccessToken: "helloworld",
			}

			bytes, err := json.Marshal(token)
			assert.NoError(t, err)
			byteResp = bytes
		}

		_, err := w.Write(byteResp)

		assert.NoError(t, err)
	}))
}

func getBrokenHTTPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		byteResp := []byte("imbroken")

		_, err := w.Write(byteResp)

		assert.NoError(t, err)
	}))
}

func TestPostgreSQL_DoMaintenance(t *testing.T) {
	tests := []struct {
		name             string
		wantErr          bool
		objs             []client.Object
		maintTimeout     time.Duration
		wantedClaim      *vshnv1.VSHNPostgreSQL
		wantedOps        *stackgresv1.SGDbOps
		server           *httptest.Server
		updatedOps       string
		shouldSkipRepack bool
	}{
		{
			name:         "GivenEOLVersion_ThenExpectEOLStatus",
			maintTimeout: time.Hour,
			objs: []client.Object{
				&stackgresv1.SGCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster",
						Namespace: "default",
					},
					Spec: stackgresv1.SGClusterSpec{
						Postgres: stackgresv1.SGClusterSpecPostgres{
							Version: "12.0",
						},
					},
				},
				&vshnv1.VSHNPostgreSQL{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "myclaim",
						Namespace: "default",
					},
					Spec: vshnv1.VSHNPostgreSQLSpec{
						Parameters: vshnv1.VSHNPostgreSQLParameters{
							Service: vshnv1.VSHNPostgreSQLServiceSpec{
								RepackEnabled: true,
							},
						},
					},
				},
			},
			wantedClaim: &vshnv1.VSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myclaim",
					Namespace: "default",
				},
				Status: vshnv1.VSHNPostgreSQLStatus{
					IsEOL: true,
				},
			},
			server: getVersionTestHTTPServer(t),
		},
		{
			name:         "GivenOlderMinorVersion_ThenExpectMinorUpdate",
			maintTimeout: time.Hour,
			objs: []client.Object{
				&stackgresv1.SGCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster",
						Namespace: "default",
					},
					Spec: stackgresv1.SGClusterSpec{
						Postgres: stackgresv1.SGClusterSpecPostgres{
							Version: "15.0",
						},
					},
				},
				&vshnv1.VSHNPostgreSQL{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "myclaim",
						Namespace: "default",
					},
					Spec: vshnv1.VSHNPostgreSQLSpec{
						Parameters: vshnv1.VSHNPostgreSQLParameters{
							Service: vshnv1.VSHNPostgreSQLServiceSpec{
								RepackEnabled: true,
							},
						},
					},
				},
			},
			wantedOps: &stackgresv1.SGDbOps{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "minorupgrade",
					Namespace: "default",
				},
				Spec: stackgresv1.SGDbOpsSpec{
					MinorVersionUpgrade: &stackgresv1.SGDbOpsSpecMinorVersionUpgrade{
						Method:          pointer.String("InPlace"),
						PostgresVersion: pointer.String("15.1"),
					},
					Op:         "minorVersionUpgrade",
					SgCluster:  "cluster",
					MaxRetries: pointer.Int(1),
				},
			},
			server: getVersionTestHTTPServer(t),
		},
		{
			name:         "GivenSameMinorVersion_ThenExpectSecurityMaintenance",
			maintTimeout: time.Hour,
			objs: []client.Object{
				&stackgresv1.SGCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster",
						Namespace: "default",
					},
					Spec: stackgresv1.SGClusterSpec{
						Postgres: stackgresv1.SGClusterSpecPostgres{
							Version: "15.1",
						},
					},
				},
				&vshnv1.VSHNPostgreSQL{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "myclaim",
						Namespace: "default",
					},
					Spec: vshnv1.VSHNPostgreSQLSpec{
						Parameters: vshnv1.VSHNPostgreSQLParameters{
							Service: vshnv1.VSHNPostgreSQLServiceSpec{
								RepackEnabled: true,
							},
						},
					},
				},
			},
			wantedOps: &stackgresv1.SGDbOps{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "securitymaintenance",
					Namespace: "default",
				},
				Spec: stackgresv1.SGDbOpsSpec{
					Op:         "securityUpgrade",
					SgCluster:  "cluster",
					MaxRetries: pointer.Int(1),
					SecurityUpgrade: &stackgresv1.SGDbOpsSpecSecurityUpgrade{
						Method: pointer.String("InPlace"),
					},
				},
			},
			server: getVersionTestHTTPServer(t),
		},
		{
			name:         "GivenUnavailableStackGresAPI_ThenExpectSecurityMaintenance",
			maintTimeout: time.Hour,
			objs: []client.Object{
				&stackgresv1.SGCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster",
						Namespace: "default",
					},
					Spec: stackgresv1.SGClusterSpec{
						Postgres: stackgresv1.SGClusterSpecPostgres{
							Version: "15.0",
						},
					},
				},
				&vshnv1.VSHNPostgreSQL{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "myclaim",
						Namespace: "default",
					},
					Spec: vshnv1.VSHNPostgreSQLSpec{
						Parameters: vshnv1.VSHNPostgreSQLParameters{
							Service: vshnv1.VSHNPostgreSQLServiceSpec{
								RepackEnabled: true,
							},
						},
					},
				},
			},
			wantedOps: &stackgresv1.SGDbOps{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "securitymaintenance",
					Namespace: "default",
				},
				Spec: stackgresv1.SGDbOpsSpec{
					Op:         "securityUpgrade",
					SgCluster:  "cluster",
					MaxRetries: pointer.Int(1),
					SecurityUpgrade: &stackgresv1.SGDbOpsSpecSecurityUpgrade{
						Method: pointer.String("InPlace"),
					},
				},
			},
			server: getBrokenHTTPServer(t),
		},
		{
			name:         "GivenMaintenanceTooLong_ThenExpectNoRepack",
			maintTimeout: 500 * time.Millisecond,
			objs: []client.Object{
				&stackgresv1.SGCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster",
						Namespace: "default",
					},
					Spec: stackgresv1.SGClusterSpec{
						Postgres: stackgresv1.SGClusterSpecPostgres{
							Version: "15.0",
						},
					},
				},
			},
			server:           getVersionTestHTTPServer(t),
			shouldSkipRepack: true,
		},
		{
			name:         "GivenMaintenanceTooLong_WithUnrelatedSecupdate_ThenExpectNoRepack",
			maintTimeout: 2 * time.Second,
			objs: []client.Object{
				&stackgresv1.SGCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster",
						Namespace: "default",
					},
					Spec: stackgresv1.SGClusterSpec{
						Postgres: stackgresv1.SGClusterSpecPostgres{
							Version: "15.0",
						},
					},
				},
				&stackgresv1.SGDbOps{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unrelated-securitymaintenance",
						Namespace: "default",
					},
					Spec: stackgresv1.SGDbOpsSpec{
						Op:         "securityUpgrade",
						SgCluster:  "cluster",
						MaxRetries: pointer.Int(1),
						SecurityUpgrade: &stackgresv1.SGDbOpsSpecSecurityUpgrade{
							Method: pointer.String("InPlace"),
						},
					},
				},
			},
			server:           getVersionTestHTTPServer(t),
			updatedOps:       "securityUpgrade",
			shouldSkipRepack: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			setupEnvVars(t)

			// Since controller-runtime v0.15.0 the fakeclient needs to be made
			// aware of object status subresources, if you want to update them.
			fclient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				WithObjects(tt.objs...).
				WithStatusSubresource(&stackgresv1.SGDbOps{}, &stackgresv1.SGCluster{}, &stackgresv1.SGPostgresConfig{}).
				Build()

			defer tt.server.Close()
			ctx := context.TODO()
			concurrentSGDbOpsUpdate(fclient, tt.updatedOps)

			p := &PostgreSQL{
				Client:       fclient,
				log:          logr.Discard(),
				SgURL:        tt.server.URL,
				MaintTimeout: tt.maintTimeout,
			}
			assert.NotPanics(t, func() {
				if err := p.DoMaintenance(ctx); (err != nil) != tt.wantErr {
					t.Errorf("PostgreSQL.DoMaintenance() error = %v, wantErr %v", err, tt.wantErr)
				}
			})

			if tt.wantedClaim != nil {

				claim := &vshnv1.VSHNPostgreSQL{}

				assert.NoError(t, fclient.Get(ctx, client.ObjectKeyFromObject(tt.wantedClaim), claim))

				assert.Equal(t, tt.wantedClaim.Status.IsEOL, claim.Status.IsEOL)
			}

			if tt.wantedOps != nil {
				ops := &stackgresv1.SGDbOps{}

				assert.NoError(t, fclient.Get(ctx, client.ObjectKeyFromObject(tt.wantedOps), ops))

				assert.Equal(t, tt.wantedOps.Spec, ops.Spec)
			}

			repack := &stackgresv1.SGDbOps{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "databasesrepack",
					Namespace: "default",
				},
			}
			if !tt.shouldSkipRepack {
				assert.NoError(t, fclient.Get(ctx, client.ObjectKeyFromObject(repack), repack))
				assert.Equal(t, "repack", repack.Spec.Op)
			} else {
				assert.Error(t, fclient.Get(ctx, client.ObjectKeyFromObject(repack), repack))
			}
		})
	}
}

func concurrentSGDbOpsUpdate(fakeClient client.WithWatch, op string) {
	go func() {
		ctx := context.Background()
		sl := &stackgresv1.SGDbOpsList{}
		watcher, err := fakeClient.Watch(ctx, sl)
		if err != nil {
			panic(err)
		}
		defer watcher.Stop()
		rc := watcher.ResultChan()

		<-rc

		sl.Items = []stackgresv1.SGDbOps{}
		// Check that we can handle not completes
		err = fakeClient.List(ctx, sl)
		if err != nil {
			panic(err)
		}
		for _, s := range sl.Items {
			err := fakeClient.Get(ctx, client.ObjectKeyFromObject(&s), &s)
			if err != nil {
				panic(err)
			}

			s.Status.Conditions = &[]stackgresv1.SGDbOpsStatusConditionsItem{
				{
					Message: pointer.String("not yet"),
					Reason:  pointer.String("OperationnNotCompleted"),
					Status:  pointer.String("False"),
					Type:    pointer.String("Completed"),
				},
			}
			err = fakeClient.Status().Update(ctx, &s)
			if err != nil {
				panic(err)
			}
		}

		// Complete only the configured sgdbop
		for {
			time.Sleep(time.Second)
			err := fakeClient.List(ctx, sl)
			if err != nil {
				panic(err)
			}
			for _, s := range sl.Items {
				if op != "" && s.Spec.Op != op {
					continue
				}
				s.Status.Conditions = &[]stackgresv1.SGDbOpsStatusConditionsItem{
					{
						Message: pointer.String("all good"),
						Reason:  pointer.String("OperationCompleted"),
						Status:  pointer.String("True"),
						Type:    pointer.String("Completed"),
					},
				}
				err := fakeClient.Status().Update(ctx, &s)
				if err != nil {
					panic(err)
				}
			}
		}
	}()
}

func setupEnvVars(t *testing.T) {
	t.Setenv("INSTANCE_NAMESPACE", "default")
	t.Setenv("API_PASSWORD", "password")
	t.Setenv("API_USERNAME", "admin")
	t.Setenv("CLAIM_NAME", "myclaim")
	t.Setenv("CLAIM_NAMESPACE", "default")

	viper.AutomaticEnv()
}
