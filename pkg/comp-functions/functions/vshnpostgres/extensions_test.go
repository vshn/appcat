package vshnpostgres

import (
	"context"
	"testing"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/stretchr/testify/assert"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"k8s.io/utils/ptr"
)

func Test_enableTimescaleDB(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		iofFile string
	}{
		{
			name:    "GivenFreshInstall_WhenEnablingTimescale_ThenExpectConfig",
			wantErr: false,
			iofFile: "vshn-postgres/extensions/01-GivenFreshConfig.yaml",
		},
		{
			name:    "GivenAlreadyEnabled_ThenExpectConfig",
			wantErr: false,
			iofFile: "vshn-postgres/extensions/01-GivenAlreadyEnabled.yaml",
		},
	}
	for _, tt := range tests {
		ctx := context.TODO()
		svc := commontest.LoadRuntimeFromFile(t, tt.iofFile)

		comp := &vshnv1.VSHNPostgreSQL{}

		err := svc.GetDesiredComposite(comp)
		if err != nil {
			t.Fatal("Can't get composite", err)
		}

		t.Log("Composite", comp.GetName())

		t.Run(tt.name, func(t *testing.T) {
			if err := enableTimescaleDB(ctx, svc, comp.GetName()); (err != nil) != tt.wantErr {
				t.Errorf("enableTimescaleDB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})

		config := &stackgresv1.SGPostgresConfig{}

		assert.NoError(t, svc.GetDesiredKubeObject(config, configResourceName))

		assert.Contains(t, config.Spec.PostgresqlConf[sharedLibraries], timescaleExtName)

		assert.Contains(t, config.Spec.PostgresqlConf["max_connections"], "200")
	}
}

func Test_disableTimescaleDB(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		iofFile string
	}{
		{
			name:    "GivenFreshInstall_WhenDisablingTimescale_ThenExpectNoConfig",
			wantErr: false,
			iofFile: "vshn-postgres/extensions/01-GivenFreshConfig.yaml",
		},
		{
			name:    "GivenAlreadyEnabled_ThenExpectNoConfig",
			wantErr: false,
			iofFile: "vshn-postgres/extensions/01-GivenAlreadyEnabled.yaml",
		},
	}
	for _, tt := range tests {

		ctx := context.TODO()
		svc := commontest.LoadRuntimeFromFile(t, tt.iofFile)

		t.Run(tt.name, func(t *testing.T) {
			if err := disableTimescaleDB(ctx, svc); (err != nil) != tt.wantErr {
				t.Errorf("disableTimescaleDB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})

		config := &stackgresv1.SGPostgresConfig{}

		assert.NoError(t, svc.GetDesiredKubeObject(config, configResourceName))

		assert.NotContains(t, config.Spec.PostgresqlConf[sharedLibraries], timescaleExtName)

	}
}

func TestAddExtensions(t *testing.T) {
	tests := []struct {
		name          string
		wantErr       bool
		iofFile       string
		want          *xfnproto.Result
		wantExensions []stackgresv1.SGClusterSpecPostgresExtensionsItem
	}{
		{
			name:    "GivenFreshInstall_WhenDisablingTimescale_ThenExpectNoConfig",
			wantErr: false,
			iofFile: "vshn-postgres/extensions/01-GivenFreshConfig.yaml",
			want:    nil,
			wantExensions: []stackgresv1.SGClusterSpecPostgresExtensionsItem{
				{
					Name:    "pg_repack",
					Version: ptr.To("1.5.0"),
				},
			},
		},
		{
			name:    "GivenAlreadyEnabled_ThenExpectNoConfig",
			wantErr: false,
			iofFile: "vshn-postgres/extensions/01-GivenAlreadyEnabled.yaml",
			want:    nil,
			wantExensions: []stackgresv1.SGClusterSpecPostgresExtensionsItem{
				{
					Name:    "pg_repack",
					Version: ptr.To("1.5.0"),
				},
			},
		},
	}
	for _, tt := range tests {

		ctx := context.TODO()
		iof := commontest.LoadRuntimeFromFile(t, tt.iofFile)

		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AddExtensions(ctx, &vshnv1.VSHNPostgreSQL{}, iof))

			cluster := &stackgresv1.SGCluster{}

			assert.NoError(t, iof.GetDesiredKubeObject(cluster, "cluster"))

			assert.Equal(t, &tt.wantExensions, cluster.Spec.Postgres.Extensions)

		})
	}
}
