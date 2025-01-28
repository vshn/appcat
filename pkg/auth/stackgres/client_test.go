package stackgres

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetLatestMinorVersion(t *testing.T) {
	tests := []struct {
		name          string
		inputVersion  string
		versionList   *PgVersions
		expected      string
		expectedError string
	}{
		{
			name:         "No version list provided",
			inputVersion: "14",
			versionList:  nil,
			expected:     "14",
		},
		{
			name:         "Empty version list",
			inputVersion: "14",
			versionList:  &PgVersions{Postgresql: []string{}},
			expected:     "14",
		},
		{
			name:         "Single matching minor version",
			inputVersion: "14",
			versionList:  &PgVersions{Postgresql: []string{"14.3"}},
			expected:     "14.3",
		},
		{
			name:         "Multiple minor versions, latest chosen",
			inputVersion: "14",
			versionList:  &PgVersions{Postgresql: []string{"14.1", "14.3", "14.4"}},
			expected:     "14.4",
		},
		{
			name:         "No matching major version",
			inputVersion: "14",
			versionList:  &PgVersions{Postgresql: []string{"13.5", "15.1"}},
			expected:     "14",
		},
		{
			name:         "Minor version is 0",
			inputVersion: "14",
			versionList:  &PgVersions{Postgresql: []string{"13.3", "14.0"}},
			expected:     "14",
		},
		{
			name:          "Invalid input version format",
			inputVersion:  "invalid-version",
			versionList:   &PgVersions{Postgresql: []string{"14.3", "14.4"}},
			expectedError: "Malformed version: invalid-version",
		},
		{
			name:          "Invalid version in version list",
			inputVersion:  "14.2",
			versionList:   &PgVersions{Postgresql: []string{"14.3", "invalid-version"}},
			expectedError: "Malformed version: invalid-version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetLatestMinorVersion(tt.inputVersion, tt.versionList)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
