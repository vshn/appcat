package release

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/compat"
)

func TestFilterRevisionsByCompatibility(t *testing.T) {
	m, err := compat.ParseMatrix([]byte("postgresql:\n  - versionRange: \"16.x\"\n    compatibleRevisions: \"<v3.73.0\"\n"))
	require.NoError(t, err)

	revisions := []string{"v3.72.0-v4.180.0", "v3.73.0-v4.190.0"}
	got := filterRevisionsByCompatibility(revisions, m, "postgresql", "16")

	// v3.73.0 violates "<v3.73.0"; only v3.72.0 remains.
	require.Len(t, got, 1)
	assert.Equal(t, "v3.72.0-v4.180.0", got[0])
}

func TestFilterRevisionsByCompatibility_emptyMatrix_keepsAll(t *testing.T) {
	revisions := []string{"v3.72.0-v4.180.0", "v3.73.0-v4.190.0"}
	got := filterRevisionsByCompatibility(revisions, compat.Matrix{}, "postgresql", "16")
	assert.Equal(t, revisions, got)
}
