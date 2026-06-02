package vshngarage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyAllowedNamespaces(t *testing.T) {
	t.Run("empty raw leaves values untouched", func(t *testing.T) {
		values := map[string]any{"existing": "keep"}
		require.NoError(t, applyAllowedNamespaces(values, ""))
		_, present := values["allowedNamespaces"]
		assert.False(t, present)
		assert.Equal(t, "keep", values["existing"])
	})

	t.Run("populated list lands in values", func(t *testing.T) {
		values := map[string]any{}
		require.NoError(t, applyAllowedNamespaces(values, `["syn-appcat","team-a"]`))
		assert.Equal(t, []string{"syn-appcat", "team-a"}, values["allowedNamespaces"])
	})

	t.Run("empty json array leaves values untouched", func(t *testing.T) {
		values := map[string]any{}
		require.NoError(t, applyAllowedNamespaces(values, `[]`))
		_, present := values["allowedNamespaces"]
		assert.False(t, present)
	})

	t.Run("malformed json fails", func(t *testing.T) {
		values := map[string]any{}
		err := applyAllowedNamespaces(values, `not-json`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot parse garageAllowedNamespaces")
	})
}
