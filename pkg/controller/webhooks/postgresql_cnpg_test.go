package webhooks

import (
	"testing"

	cpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCNPGValidator_PgConf(t *testing.T) {
	tests := []struct {
		name        string
		settings    string
		expectErr   bool
		errContains string
	}{
		{
			name:      "GivenNoSettings_ThenNoError",
			settings:  "",
			expectErr: false,
		},
		{
			name:      "GivenAllowedSetting_ThenNoError",
			settings:  `{"max_connections": "100"}`,
			expectErr: false,
		},
		{
			name:        "GivenFixedParameter_ThenError",
			settings:    `{"archive_mode": "off"}`,
			expectErr:   true,
			errContains: "cloudnative-pg.io/docs",
		},
		{
			name:        "GivenSharedPreloadLibraries_ThenError",
			settings:    `{"shared_preload_libraries": "pg_stat_statements"}`,
			expectErr:   true,
			errContains: "spec.parameters.service.postgresqlSettings[shared_preload_libraries]",
		},
		{
			name:      "GivenStackGresOnlyBlockedSetting_ThenNoError",
			settings:  `{"fsync": "off", "wal_level": "logical", "max_wal_senders": "20"}`,
			expectErr: false,
		},
		{
			name:        "GivenInvalidJSON_ThenError",
			settings:    `{"broken"`,
			expectErr:   true,
			errContains: "error parsing pgConf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := &vshnv1.VSHNPostgreSQL{
				Spec: vshnv1.VSHNPostgreSQLSpec{
					CompositionRef: cpv1.CompositionReference{Name: cnpgCompositionRef},
				},
			}
			if tt.settings != "" {
				pg.Spec.Parameters.Service.PostgreSQLSettings = runtime.RawExtension{Raw: []byte(tt.settings)}
			}

			errs := cnpgValidator{}.validate(pg, nil)

			if tt.expectErr {
				assert.NotEmpty(t, errs)
				assert.Contains(t, errs.ToAggregate().Error(), tt.errContains)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestCNPGValidator_Extensions(t *testing.T) {
	tests := []struct {
		name         string
		majorVersion string
		extensions   []vshnv1.VSHNDBaaSPostgresExtension
		expectErr    bool
		errContains  string
	}{
		{
			name:         "GivenNoExtensions_ThenNoError",
			majorVersion: "18",
			expectErr:    false,
		},
		{
			name:         "GivenExtensionWithoutImageFields_ThenNoError",
			majorVersion: "17",
			extensions:   []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector"}},
			expectErr:    false,
		},
		{
			name:         "GivenExtensionWithImageAndVersion18_ThenNoError",
			majorVersion: "18",
			extensions:   []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector", Image: "ghcr.io/vshn/pgvector:latest"}},
			expectErr:    false,
		},
		{
			name:         "GivenExtensionWithImagePullPolicyAndVersion18_ThenNoError",
			majorVersion: "18",
			extensions:   []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector", ImagePullPolicy: "Always"}},
			expectErr:    false,
		},
		{
			name:         "GivenExtensionWithImageAndVersion19_ThenNoError",
			majorVersion: "19",
			extensions:   []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector", Image: "ghcr.io/vshn/pgvector:latest"}},
			expectErr:    false,
		},
		{
			name:         "GivenExtensionWithImageAndVersionBelow18_ThenError",
			majorVersion: "17",
			extensions:   []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector", Image: "ghcr.io/vshn/pgvector:latest"}},
			expectErr:    true,
			errContains:  "image is only supported for PostgreSQL 18 and above",
		},
		{
			name:         "GivenExtensionWithImagePullPolicyAndVersionBelow18_ThenError",
			majorVersion: "16",
			extensions:   []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector", ImagePullPolicy: "Always"}},
			expectErr:    true,
			errContains:  "imagePullPolicy is only supported for PostgreSQL 18 and above",
		},
		{
			name:         "GivenExtensionWithImageAndUnparseableVersion_ThenError",
			majorVersion: "foo",
			extensions:   []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector", Image: "ghcr.io/vshn/pgvector:latest"}},
			expectErr:    true,
			errContains:  "image is only supported for PostgreSQL 18 and above",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := &vshnv1.VSHNPostgreSQL{
				Spec: vshnv1.VSHNPostgreSQLSpec{
					CompositionRef: cpv1.CompositionReference{Name: cnpgCompositionRef},
					Parameters: vshnv1.VSHNPostgreSQLParameters{
						Service: vshnv1.VSHNPostgreSQLServiceSpec{
							MajorVersion: tt.majorVersion,
							Extensions:   tt.extensions,
						},
					},
				},
			}

			errs := cnpgValidator{}.validate(pg, nil)

			if tt.expectErr {
				assert.NotEmpty(t, errs)
				assert.Contains(t, errs.ToAggregate().Error(), tt.errContains)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestCNPGValidator_MajorVersion(t *testing.T) {
	tests := []struct {
		name           string
		newVersion     string
		currentVersion string
		expectErr      bool
		errContains    string
	}{
		{
			name:           "GivenSameVersion_ThenNoError",
			newVersion:     "15",
			currentVersion: "15",
			expectErr:      false,
		},
		{
			name:           "GivenMajorUpgrade_ThenNoError",
			newVersion:     "16",
			currentVersion: "15",
			expectErr:      false,
		},
		{
			name:           "GivenMajorUpgradeFromFullCurrentVersion_ThenNoError",
			newVersion:     "16",
			currentVersion: "15.9",
			expectErr:      false,
		},
		{
			name:           "GivenNoCurrentVersion_ThenNoError",
			newVersion:     "16",
			currentVersion: "",
			expectErr:      false,
		},
		{
			name:           "GivenMajorDowngrade_ThenError",
			newVersion:     "14",
			currentVersion: "15",
			expectErr:      true,
			errContains:    "downgrading from",
		},
		{
			name:           "GivenMajorDowngradeFromFullCurrentVersion_ThenError",
			newVersion:     "14",
			currentVersion: "15.9",
			expectErr:      true,
			errContains:    "downgrading from",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPg := &vshnv1.VSHNPostgreSQL{
				Spec: vshnv1.VSHNPostgreSQLSpec{
					CompositionRef: cpv1.CompositionReference{Name: cnpgCompositionRef},
					Parameters: vshnv1.VSHNPostgreSQLParameters{
						Service: vshnv1.VSHNPostgreSQLServiceSpec{MajorVersion: tt.newVersion},
					},
				},
			}
			oldPg := &vshnv1.VSHNPostgreSQL{
				Status: vshnv1.VSHNPostgreSQLStatus{CurrentVersion: tt.currentVersion},
			}

			errs := cnpgValidator{}.validate(newPg, oldPg)

			if tt.expectErr {
				assert.NotEmpty(t, errs)
				assert.Contains(t, errs.ToAggregate().Error(), tt.errContains)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestCNPGBlocklistMatchesUpstream(t *testing.T) {
	// The list mirrors the fixed parameters documented by CloudNativePG.
	assert.Len(t, cnpgBlocklist, 62)

	for _, key := range []string{"archive_mode", "listen_addresses", "port", "ssl_key_file", "unix_socket_permissions"} {
		_, blocked := cnpgBlocklist[key]
		assert.True(t, blocked, "%s must be blocked", key)
	}

	for _, key := range []string{"wal_level", "fsync", "max_connections", "shared_buffers"} {
		_, blocked := cnpgBlocklist[key]
		assert.False(t, blocked, "%s must not be blocked", key)
	}
}
