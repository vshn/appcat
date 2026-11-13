package vshnpostgrescnpg

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func TestClusterOfflineAlertFiresWhenSeriesAreAbsent(t *testing.T) {
	r := clusterOfflineAlert("postgres", "vshn-postgresql-test")
	// pods gone means no cnpg_collector_up series at all, so only a vector(0)
	// fallback can ever reach == 0
	assert.Contains(t, r.Expr.StrVal, "OR on() vector(0)")
	assert.Contains(t, r.Expr.StrVal, `namespace="vshn-postgresql-test"`)
}

func TestConnectionsCriticalCountsAllBackends(t *testing.T) {
	r := maxConnectionsAlert("postgres", "vshn-postgresql-test")
	// the metrics exporter's own connection is not the customer's connection usage
	assert.NotContains(t, r.Expr.StrVal, "application_name")
	assert.Equal(t, `sum by (pod) (cnpg_backends_total{namespace="vshn-postgresql-test"})
  / max by (pod) (cnpg_pg_settings_setting{name="max_connections", namespace="vshn-postgresql-test"})
  * 100 > 90`, r.Expr.StrVal)

	// same shape as the warning, only the threshold differs
	warning := highConnectionsWarningAlert("postgres", "vshn-postgresql-test")
	assert.Equal(t, strings.TrimSuffix(warning.Expr.StrVal, "80"), strings.TrimSuffix(r.Expr.StrVal, "90"))
}

func TestDeadlockConflictsUsesIncrease(t *testing.T) {
	r := deadlockConflictsAlert("postgres", "vshn-postgresql-test")
	// cnpg_pg_stat_database_deadlocks is a counter since the last stats reset;
	// increase() gives the deadlocks of the last 5 minutes and handles counter resets
	assert.Equal(t, `increase(cnpg_pg_stat_database_deadlocks{namespace="vshn-postgresql-test"}[5m]) > 10`, r.Expr.StrVal)
}

func renderNonSLARules(t *testing.T, fixture string) map[string]int {
	svc := commontest.LoadRuntimeFromFile(t, fixture)
	assert.Nil(t, generateNonSLAPromRules(context.TODO(), &vshnv1.VSHNPostgreSQL{}, svc))

	kubeObject := &xkube.Object{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObject, "psql-non-slo-rules"))
	pr := &promv1.PrometheusRule{}
	assert.NoError(t, json.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, pr))

	alerts := map[string]int{}
	for _, r := range pr.Spec.Groups[0].Rules {
		alerts[r.Alert]++
	}
	return alerts
}

func TestNonSLARulesRunningInstance(t *testing.T) {
	alerts := renderNonSLARules(t, "vshn-postgres/alerting/13-GivenCnpgRunning.yaml")
	assert.Equal(t, 1, alerts["CNPGClusterOffline"])
	// AddAll() used to pick up StackGres' "maxconnections" as well
	assert.Equal(t, 1, alerts["PostgreSQLConnectionsCritical"])
	for a, n := range alerts {
		assert.Equal(t, 1, n, a)
	}
	assert.Len(t, alerts, 12)
}

func TestNonSLARulesHibernatedInstance(t *testing.T) {
	alerts := renderNonSLARules(t, "vshn-postgres/alerting/14-GivenCnpgHibernated.yaml")
	// no pods run while hibernated, so CNPGClusterOffline would always fire
	assert.NotContains(t, alerts, "CNPGClusterOffline")
	assert.Len(t, alerts, 11)
}
