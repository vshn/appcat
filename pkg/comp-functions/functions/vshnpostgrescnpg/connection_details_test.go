package vshnpostgrescnpg

import (
	"context"
	"testing"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

	"github.com/stretchr/testify/assert"
)

func TestTransform_NoInstanceNamespace(t *testing.T) {
	ctx := context.Background()
	expectSvc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/secrets/01_expected_no-instance-namespace.yaml")
	expectResult := runtime.NewWarningResult("cannot get the sgcluster object: not found")

	t.Run("WhenNoInstance_ThenWarningAndNoChange", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/secrets/01_input_no-instance-namespace.yaml")

		// When
		result := AddConnectionDetails(ctx, &vshnv1.VSHNPostgreSQLCNPG{}, svc)

		// Then
		assert.Equal(t, expectResult, result)
		assert.Equal(t, expectSvc, svc)
	})
}

func TestGetPostgresURL(t *testing.T) {
	tests := map[string]struct {
		secret    map[string][]byte
		password  string
		expectURL string
	}{
		"WhenMissingPasswordThenReturnNoUrlInSecret": {
			secret: map[string][]byte{
				PostgresqlDb:   []byte("db-test"),
				PostgresqlHost: []byte("localhost"),
				PostgresqlUser: []byte("user"),
				PostgresqlPort: []byte("5432"),
			},
			password:  "",
			expectURL: "",
		},
		"WhenDataThenReturnSecretWithUrl": {
			secret: map[string][]byte{
				PostgresqlPassword: []byte("test"),
			},
			password:  "test",
			expectURL: "postgres://postgres:test@localhost:5432/postgres",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// When
			url := getPostgresURL("localhost", tc.password)

			// Then
			assert.Equal(t, tc.expectURL, url)
		})
	}
}

func TestGetPostgresURLCustomUser(t *testing.T) {
	tests := map[string]struct {
		db        string
		user      string
		password  string
		expectURL string
	}{
		"WhenMissingPasswordThenReturnNoUrlInSecret": {
			db:        "db-test",
			user:      "user",
			password:  "",
			expectURL: "",
		},
		"WhenDataThenReturnSecretWithUrl": {
			db:        "db-test",
			user:      "user",
			password:  "test",
			expectURL: "postgres://user:test@localhost:5432/db-test",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// When
			url := getPostgresURLCustomUser("localhost", tc.user, tc.password, tc.db)

			// Then
			assert.Equal(t, tc.expectURL, url)
		})
	}
}
