package vshnpostgres

import (
	"context"
	"testing"

	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestTransform_NoInstanceNamespace(t *testing.T) {
	ctx := context.Background()
	expectSvc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/url/01_expected_no-instance-namespace.yaml")
	expectResult := runtime.NewWarningResult("Composite is missing instance namespace, skipping transformation")

	t.Run("WhenNoInstance_ThenNoErrorAndNoChanges", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/url/01_input_no-instance-namespace.yaml")

		// When
		result := AddUrlToConnectionDetails(ctx, svc)

		// Then
		assert.Equal(t, expectResult, result)
		assert.Equal(t, expectSvc, svc)
	})
}

func TestTransform(t *testing.T) {
	ctx := context.Background()
	expectURL := "postgres://postgres:639b-9076-4de6-a35@" +
		"pgsql-gc9x4.vshn-postgresql-pgsql-gc9x4.svc.cluster.local:5432/postgres"

	t.Run("WhenNormalIO_ThenAddPostgreSQLUrl", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/url/02_input_function-io.yaml")

		// When
		result := AddUrlToConnectionDetails(ctx, svc)

		// Then
		assert.Nil(t, result)
		cd := svc.GetConnectionDetails()
		assert.Equal(t, expectURL, string(cd["POSTGRESQL_URL"]))
	})
}

func TestGetPostgresURL(t *testing.T) {
	tests := map[string]struct {
		secret    *v1.Secret
		expectURL string
	}{
		"WhenMissingUserAndPortThenReturnNoUrlInSecret": {
			secret: &v1.Secret{
				Data: map[string][]byte{
					PostgresqlPassword: []byte("test"),
					PostgresqlDb:       []byte("db-test"),
					PostgresqlHost:     []byte("localhost"),
				},
			},
			expectURL: "",
		},
		"WhenMissingPasswordThenReturnNoUrlInSecret": {
			secret: &v1.Secret{
				Data: map[string][]byte{
					PostgresqlDb:   []byte("db-test"),
					PostgresqlHost: []byte("localhost"),
					PostgresqlUser: []byte("user"),
					PostgresqlPort: []byte("5432"),
				},
			},
			expectURL: "",
		},
		"WhenDataThenReturnSecretWithUrl": {
			secret: &v1.Secret{
				Data: map[string][]byte{
					PostgresqlPassword: []byte("test"),
					PostgresqlDb:       []byte("db-test"),
					PostgresqlHost:     []byte("localhost"),
					PostgresqlUser:     []byte("user"),
					PostgresqlPort:     []byte("5432"),
				},
				StringData: map[string]string{
					"place": "test",
				},
			},
			expectURL: "postgres://user:test@localhost:5432/db-test",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// When
			url := getPostgresURL(tc.secret)

			// Then
			assert.Equal(t, tc.expectURL, url)
		})
	}
}
