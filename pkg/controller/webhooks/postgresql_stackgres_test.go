package webhooks

import (
	"testing"

	cpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestStackGresValidator_PgConf(t *testing.T) {
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
			name:        "GivenBlockedSetting_ThenError",
			settings:    `{"fsync": "off"}`,
			expectErr:   true,
			errContains: "stackgres.io/doc",
		},
		{
			name:        "GivenMixedSettings_ThenError",
			settings:    `{"max_connections": "100", "wal_level": "minimal"}`,
			expectErr:   true,
			errContains: "spec.parameters.service.postgresqlSettings[wal_level]",
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
			pg := &vshnv1.VSHNPostgreSQL{}
			if tt.settings != "" {
				pg.Spec.Parameters.Service.PostgreSQLSettings = runtime.RawExtension{Raw: []byte(tt.settings)}
			}

			errs := stackgresValidator{}.validate(pg, nil)

			if tt.expectErr {
				assert.NotEmpty(t, errs)
				assert.Contains(t, errs.ToAggregate().Error(), tt.errContains)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestStackGresValidator_Extensions(t *testing.T) {
	tests := []struct {
		name        string
		extensions  []vshnv1.VSHNDBaaSPostgresExtension
		expectErr   bool
		errContains string
	}{
		{
			name:       "GivenNoExtensions_ThenNoError",
			extensions: nil,
			expectErr:  false,
		},
		{
			name:       "GivenExtensionWithoutImageFields_ThenNoError",
			extensions: []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector"}},
			expectErr:  false,
		},
		{
			name:        "GivenExtensionWithImage_ThenError",
			extensions:  []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector", Image: "ghcr.io/vshn/pgvector:latest"}},
			expectErr:   true,
			errContains: "image is only supported for CloudNativePG",
		},
		{
			name:        "GivenExtensionWithImagePullPolicy_ThenError",
			extensions:  []vshnv1.VSHNDBaaSPostgresExtension{{Name: "pgvector", ImagePullPolicy: "IfNotPresent"}},
			expectErr:   true,
			errContains: "imagePullPolicy is only supported for CloudNativePG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := &vshnv1.VSHNPostgreSQL{
				Spec: vshnv1.VSHNPostgreSQLSpec{
					CompositionRef: cpv1.CompositionReference{Name: "vshnpostgres.vshn.appcat.vshn.io"},
					Parameters: vshnv1.VSHNPostgreSQLParameters{
						Service: vshnv1.VSHNPostgreSQLServiceSpec{
							MajorVersion: "18",
							Extensions:   tt.extensions,
						},
					},
				},
			}

			errs := stackgresValidator{}.validate(pg, nil)

			if tt.expectErr {
				assert.NotEmpty(t, errs)
				assert.Contains(t, errs.ToAggregate().Error(), tt.errContains)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestStackGresValidator_MajorVersion(t *testing.T) {
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
			name:           "GivenSameVersionAsFullCurrentVersion_ThenNoError",
			newVersion:     "15",
			currentVersion: "15.9",
			expectErr:      false,
		},
		{
			name:           "GivenNoCurrentVersion_ThenNoError",
			newVersion:     "15",
			currentVersion: "",
			expectErr:      false,
		},
		{
			name:           "GivenMajorUpgrade_ThenError",
			newVersion:     "16",
			currentVersion: "15",
			expectErr:      true,
			errContains:    "major version upgrade is not allowed",
		},
		{
			name:           "GivenMajorDowngrade_ThenError",
			newVersion:     "14",
			currentVersion: "15",
			expectErr:      true,
			errContains:    "major version upgrade is not allowed",
		},
		{
			name:           "GivenUnparseableNewVersion_ThenError",
			newVersion:     "foo",
			currentVersion: "15",
			expectErr:      true,
			errContains:    "invalid major version",
		},
		{
			name:           "GivenUnparseableCurrentVersion_ThenError",
			newVersion:     "15",
			currentVersion: "foo",
			expectErr:      true,
			errContains:    "invalid major version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPg := &vshnv1.VSHNPostgreSQL{
				Spec: vshnv1.VSHNPostgreSQLSpec{
					CompositionRef: cpv1.CompositionReference{Name: "vshnpostgres.vshn.appcat.vshn.io"},
					Parameters: vshnv1.VSHNPostgreSQLParameters{
						Service: vshnv1.VSHNPostgreSQLServiceSpec{MajorVersion: tt.newVersion},
					},
				},
			}
			oldPg := &vshnv1.VSHNPostgreSQL{
				Status: vshnv1.VSHNPostgreSQLStatus{CurrentVersion: tt.currentVersion},
			}

			errs := stackgresValidator{}.validate(newPg, oldPg)

			if tt.expectErr {
				assert.NotEmpty(t, errs)
				assert.Contains(t, errs.ToAggregate().Error(), tt.errContains)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestStackGresValidator_MajorVersionNotCheckedOnCreate(t *testing.T) {
	pg := &vshnv1.VSHNPostgreSQL{
		Spec: vshnv1.VSHNPostgreSQLSpec{
			CompositionRef: cpv1.CompositionReference{Name: "vshnpostgres.vshn.appcat.vshn.io"},
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Service: vshnv1.VSHNPostgreSQLServiceSpec{MajorVersion: "16"},
			},
		},
	}

	assert.Empty(t, stackgresValidator{}.validate(pg, nil))
}
