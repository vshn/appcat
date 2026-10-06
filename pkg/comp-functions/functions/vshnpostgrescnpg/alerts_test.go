package vshnpostgrescnpg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterOfflineAlertFiresWhenSeriesAreAbsent(t *testing.T) {
	r := clusterOfflineAlert("postgres", "vshn-postgresql-test")
	// pods gone means no cnpg_collector_up series at all, so only a vector(0)
	// fallback can ever reach == 0
	assert.Contains(t, r.Expr.StrVal, "OR on() vector(0)")
	assert.Contains(t, r.Expr.StrVal, `namespace="vshn-postgresql-test"`)
}
