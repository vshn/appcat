package nonsla

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	promV1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

var (
	// PostgreSQL alerts - most specific as currently those are the only ones where we have different container name and namespace
	patroniPersistentVolumeExpectedToFillUp = `label_replace( ((kubelet_volume_stats_available_bytes{job="kubelet",metrics_path="/metrics", namespace="vshn-postgresql-test"} / kubelet_volume_stats_capacity_bytes{job="kubelet",metrics_path="/metrics", namespace="vshn-postgresql-test"}) < 0.15 and kubelet_volume_stats_used_bytes{job="kubelet",metrics_path="/metrics", namespace="vshn-postgresql-test"} > 0 and predict_linear(max_over_time(kubelet_volume_stats_available_bytes{job="kubelet",metrics_path="/metrics", namespace="vshn-postgresql-test"}[12h])[24h:5m], 4 * 24 * 3600) < 0  unless on(namespace, persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{namespace="vshn-postgresql-test", access_mode="ReadOnlyMany"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{namespace="vshn-postgresql-test", label_excluded_from_alerts="true"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels{namespace="vshn-postgresql-test"}, "name", "$1", "namespace","(vshn-postgresql-test)")`

	patroniMemoryCritical = `label_replace( ((max(container_memory_rss{container="patroni", namespace="vshn-postgresql-test"})without (name, id)  / on(container,pod,namespace)  kube_pod_container_resource_limits{container="patroni", namespace="vshn-postgresql-test", resource="memory"}* 100) > 85) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels{namespace="vshn-postgresql-test"}, "name", "$1", "namespace","(vshn-postgresql-test)")`

	patroniPersistentVolumeFillingUp = `label_replace( ((kubelet_volume_stats_available_bytes{job="kubelet", metrics_path="/metrics", namespace="vshn-postgresql-test"} / kubelet_volume_stats_capacity_bytes{job="kubelet",metrics_path="/metrics", namespace="vshn-postgresql-test"}) < 0.03 and kubelet_volume_stats_used_bytes{job="kubelet",metrics_path="/metrics", namespace="vshn-postgresql-test"} > 0 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{namespace="vshn-postgresql-test", access_mode="ReadOnlyMany"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{namespace="vshn-postgresql-test", label_excluded_from_alerts="true"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels{namespace="vshn-postgresql-test"}, "name", "$1", "namespace","(vshn-postgresql-test)")`

	mariadbPersistentVolumeFillingUp = `label_replace( ((kubelet_volume_stats_available_bytes{job="kubelet", metrics_path="/metrics", namespace="vshn-mariadb-myinstance"} / kubelet_volume_stats_capacity_bytes{job="kubelet",metrics_path="/metrics", namespace="vshn-mariadb-myinstance"}) < 0.03 and kubelet_volume_stats_used_bytes{job="kubelet",metrics_path="/metrics", namespace="vshn-mariadb-myinstance"} > 0 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{namespace="vshn-mariadb-myinstance", access_mode="ReadOnlyMany"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{namespace="vshn-mariadb-myinstance", label_excluded_from_alerts="true"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels{namespace="vshn-mariadb-myinstance"}, "name", "$1", "namespace","(vshn-mariadb-myinstance)")`

	keycloakMemoryCritical = `label_replace( ((max(container_memory_rss{container="keycloak", namespace="vshn-keycloak-myinstance"})without (name, id)  / on(container,pod,namespace)  kube_pod_container_resource_limits{container="keycloak", namespace="vshn-keycloak-myinstance", resource="memory"}* 100) > 85) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels{namespace="vshn-keycloak-myinstance"}, "name", "$1", "namespace","(vshn-keycloak-myinstance)")`

	oomEventsCritical = `label_replace( (max(increase(container_oom_events_total{container="patroni", namespace="vshn-postgresql-test"}[15m])) without (name, id) > 0) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels{namespace="vshn-postgresql-test"}, "name", "$1", "namespace","(vshn-postgresql-test)")`

	// metricSelectorRe matches every metric name used in the non-SLA expressions, including the
	// character that follows it, so we can make sure each one carries a label selector.
	metricSelectorRe = regexp.MustCompile(`\b((?:kubelet|kube|container)_[a-z_]+)(\{[^}]*\}|.)`)
)

func TestNewAlertSetBuilder(t *testing.T) {
	containerName := "patroni"
	namespace := "vshn-postgresql-test"
	builder := NewAlertSetBuilder(containerName)

	assert.NotNil(t, builder)
	assert.Equal(t, containerName, builder.as.alertContainerName)
	assert.Empty(t, builder.as.customRules)
	assert.Empty(t, builder.as.alerts)

	builder.AddAll()
	// if this test fail, please simply update order and count of alerts in this slice
	expectedAlerts := []alert{pvFillUp, pvExpectedFillUp, memCritical, oomEvents}

	// this check will break if someone adds new alert to map, but no method to Builder
	assert.Equal(t, len(builder.as.alerts), len(expectedAlerts))

	rules := make([]promV1.Rule, 0)
	for _, a := range builder.as.alerts {
		f := alertDefinitions[a]
		r := f(builder.as.alertContainerName, namespace)
		rules = append(rules, r)
	}
	rules = append(rules, builder.as.customRules...)

	checkCount := 0
	for _, rule := range rules {
		if rule.Alert == "PersistentVolumeExpectedToFillUp" {
			assert.Equal(t, patroniPersistentVolumeExpectedToFillUp, rule.Expr.StrVal)
			checkCount++
		}
		if rule.Alert == "MemoryCritical" {
			assert.Equal(t, patroniMemoryCritical, rule.Expr.StrVal)
			checkCount++
		}
		if rule.Alert == "PersistentVolumeFillingUp" {
			assert.Equal(t, patroniPersistentVolumeFillingUp, rule.Expr.StrVal)
			checkCount++
		}
		if rule.Alert == "MemoryOOMKilled" {
			assert.Equal(t, oomEventsCritical, rule.Expr.StrVal)
			checkCount++
		}

	}
	// working with arrays is not the best way, but it's enough for this test
	assert.Equal(t, 4, checkCount)

	// test for mariadb
	containerName = "mariadb-galera"
	namespace = "vshn-mariadb-myinstance"
	builder = NewAlertSetBuilder(containerName)
	builder.AddDiskFillingUp()

	rules = make([]promV1.Rule, 0)
	for _, a := range builder.as.alerts {
		f := alertDefinitions[a]
		r := f(builder.as.alertContainerName, namespace)
		rules = append(rules, r)
	}
	assert.Equal(t, len(rules), 1)
	assert.Equal(t, rules[0].Expr.StrVal, mariadbPersistentVolumeFillingUp)

	// test for keycloak
	containerName = "keycloak"
	namespace = "vshn-keycloak-myinstance"
	builder = NewAlertSetBuilder(containerName)
	builder.AddMemory()

	rules = make([]promV1.Rule, 0)
	for _, a := range builder.as.alerts {
		f := alertDefinitions[a]
		r := f(builder.as.alertContainerName, namespace)
		rules = append(rules, r)
	}
	assert.Equal(t, len(rules), 1)
	assert.Equal(t, rules[0].Expr.StrVal, keycloakMemoryCritical)

}

func TestGenerateNonSLAPromRulesStampsNamespace(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/07-GivenEmail.yaml")
	alerts := NewAlertSetBuilder("postgres").
		AddAll().
		AddCustom([]promV1.Rule{{Alert: "Prebuilt"}}).
		GetAlerts()

	fn := GenerateNonSLAPromRules[*vshnv1.VSHNPostgreSQL](alerts)
	assert.Nil(t, fn(context.TODO(), &vshnv1.VSHNPostgreSQL{}, svc))

	kubeObject := &xkube.Object{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObject, "psql-non-slo-rules"))
	pr := &promV1.PrometheusRule{}
	assert.NoError(t, json.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, pr))

	rules := pr.Spec.Groups[0].Rules
	assert.NotEmpty(t, rules)
	for _, r := range rules {
		assert.Equal(t, "vshn-postgresql-psql", r.Labels["namespace"], r.Alert)
	}
}

func TestNonSLAExpressionsAreNamespaceScoped(t *testing.T) {
	namespace := "vshn-forgejo-si-5181672d-5sfcm"
	nsMatcher := `namespace="` + namespace + `"`

	for a, def := range alertDefinitions {
		r := def("forgejo", namespace)
		expr := r.Expr.StrVal

		// topk/bottomk can switch between series (PVCs or pods) from one evaluation to the next.
		assert.NotContains(t, expr, "topk", a)
		assert.NotContains(t, expr, "bottomk", a)

		matches := metricSelectorRe.FindAllStringSubmatch(expr, -1)
		assert.NotEmpty(t, matches, a)
		for _, m := range matches {
			assert.Truef(t, strings.HasPrefix(m[2], "{"), "%s: metric %s has no label selector", a, m[1])
			assert.Containsf(t, m[2], nsMatcher, "%s: metric %s is not scoped to the instance namespace", a, m[1])
		}

		assert.Equal(t, strings.Count(expr, "("), strings.Count(expr, ")"), "%s: unbalanced parentheses", a)
	}
}

func TestPersistentVolumeExpectedToFillUpUsesBaselineForecast(t *testing.T) {
	expr := alertDefinitions[pvExpectedFillUp]("forgejo", "vshn-forgejo-test").Expr.StrVal
	assert.Contains(t, expr, `predict_linear(max_over_time(kubelet_volume_stats_available_bytes{job="kubelet",metrics_path="/metrics", namespace="vshn-forgejo-test"}[12h])[24h:5m], 4 * 24 * 3600) < 0`)
	assert.NotContains(t, expr, "[6h]")
}
