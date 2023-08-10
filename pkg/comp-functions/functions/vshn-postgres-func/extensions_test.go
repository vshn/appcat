package vshnpostgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
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
		iof := commontest.LoadRuntimeFromFile(t, tt.iofFile)

		t.Run(tt.name, func(t *testing.T) {
			if err := enableTimescaleDB(ctx, iof); (err != nil) != tt.wantErr {
				t.Errorf("enableTimescaleDB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})

		config := &stackgresv1.SGPostgresConfig{}

		assert.NoError(t, iof.Desired.GetFromObject(ctx, config, configResourceName))

		assert.Contains(t, config.Spec.PostgresqlConf[sharedLibraries], timescaleExtName)
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
		iof := commontest.LoadRuntimeFromFile(t, tt.iofFile)

		t.Run(tt.name, func(t *testing.T) {
			if err := disableTimescaleDB(ctx, iof); (err != nil) != tt.wantErr {
				t.Errorf("disableTimescaleDB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})

		config := &stackgresv1.SGPostgresConfig{}

		assert.NoError(t, iof.Desired.GetFromObject(ctx, config, configResourceName))

		assert.NotContains(t, config.Spec.PostgresqlConf[sharedLibraries], timescaleExtName)

	}
}

func TestAddExtensions(t *testing.T) {
	tests := []struct {
		name          string
		wantErr       bool
		iofFile       string
		want          runtime.Result
		wantExensions []stackgresv1.SGClusterSpecPostgresExtensionsItem
	}{
		{
			name:    "GivenFreshInstall_WhenDisablingTimescale_ThenExpectNoConfig",
			wantErr: false,
			iofFile: "vshn-postgres/extensions/01-GivenFreshConfig.yaml",
			want:    runtime.NewNormal(),
			wantExensions: []stackgresv1.SGClusterSpecPostgresExtensionsItem{
				{
					Name: "ltree",
				},
				{
					Name: "pg_repack",
				},
			},
		},
		{
			name:    "GivenAlreadyEnabled_ThenExpectNoConfig",
			wantErr: false,
			iofFile: "vshn-postgres/extensions/01-GivenAlreadyEnabled.yaml",
			want:    runtime.NewNormal(),
			wantExensions: []stackgresv1.SGClusterSpecPostgresExtensionsItem{
				{
					Name: "ltree",
				},
				{
					Name: "pg_repack",
				},
			},
		},
	}
	for _, tt := range tests {

		ctx := context.TODO()
		iof := commontest.LoadRuntimeFromFile(t, tt.iofFile)

		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AddExtensions(ctx, iof))

			cluster := &stackgresv1.SGCluster{}

			assert.NoError(t, iof.Desired.GetFromObject(ctx, cluster, "cluster"))

			assert.Equal(t, cluster.Spec.Postgres.Extensions, &tt.wantExensions)

		})
	}
}
