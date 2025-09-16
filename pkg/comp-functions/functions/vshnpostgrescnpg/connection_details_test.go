package vshnpostgrescnpg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
