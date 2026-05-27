package compat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMatrix(t *testing.T) {
	raw := []byte(`
postgresql:
  - versionRange: "16.x"
    compatibleRevisions: ">=v3.70.0"
  - versionRange: "<=14.x"
    compatibleRevisions: "<v3.72.0"
redis:
  - versionRange: "7.2"
    compatibleRevisions: ">=v3.60.0"
`)

	m, err := ParseMatrix(raw)
	require.NoError(t, err)
	assert.Len(t, m["postgresql"], 2)
	assert.Equal(t, "16.x", m["postgresql"][0].VersionRange)
	assert.Equal(t, ">=v3.70.0", m["postgresql"][0].CompatibleRevisions)
	assert.Len(t, m["redis"], 1)
}

func TestParseMatrix_empty(t *testing.T) {
	m, err := ParseMatrix([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestParseMatrix_invalid(t *testing.T) {
	_, err := ParseMatrix([]byte("not: [valid"))
	assert.Error(t, err)
}

func testMatrix(t *testing.T) Matrix {
	t.Helper()
	m, err := ParseMatrix([]byte(`
postgresql:
  - versionRange: "16.x"
    compatibleRevisions: ">=v3.70.0"
  - versionRange: "<=14.x"
    compatibleRevisions: "<v3.72.0"
`))
	require.NoError(t, err)
	return m
}

func TestVerdict_compatible(t *testing.T) {
	// pg 16 on revision v3.72.2-v4.186.2 -> component v3.72.2 satisfies ">=v3.70.0"
	r := Verdict(testMatrix(t), "v3.72.2-v4.186.2", "postgresql", "16")
	assert.True(t, r.Compatible)
}

func TestVerdict_incompatible(t *testing.T) {
	// pg 14 on revision v3.73.0-... -> "<=14.x" row requires "<v3.72.0"; v3.73.0 violates
	r := Verdict(testMatrix(t), "v3.73.0-v4.190.0", "postgresql", "14")
	assert.False(t, r.Compatible)
	assert.NotEmpty(t, r.Reason)
	assert.NotEmpty(t, r.Action)
}

func TestVerdict_noMatchingService_failOpen(t *testing.T) {
	r := Verdict(testMatrix(t), "v3.72.2-v4.186.2", "redis", "7.2")
	assert.True(t, r.Compatible)
}

func TestVerdict_noMatchingVersionRow_failOpen(t *testing.T) {
	// pg 15 matches neither "16.x" nor "<=14.x"
	r := Verdict(testMatrix(t), "v3.72.2-v4.186.2", "postgresql", "15")
	assert.True(t, r.Compatible)
}

func TestVerdict_unparseableRevision_failOpen(t *testing.T) {
	r := Verdict(testMatrix(t), "garbage", "postgresql", "16")
	assert.True(t, r.Compatible)
}

func TestVerdict_unparseableVersion_failOpen(t *testing.T) {
	r := Verdict(testMatrix(t), "v3.72.2-v4.186.2", "postgresql", "notaversion")
	assert.True(t, r.Compatible)
}
