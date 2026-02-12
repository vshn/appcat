package nonsla

import (
	"testing"

	promV1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
)

var (
	// PostgreSQL alerts - most specific as currently those are the only ones where we have different container name and namespace
	patroniPersistentVolumeExpectedToFillUp = `label_replace( bottomk(1, (kubelet_volume_stats_available_bytes{job="kubelet",metrics_path="/metrics"} / kubelet_volume_stats_capacity_bytes{job="kubelet",metrics_path="/metrics"}) < 0.15 and kubelet_volume_stats_used_bytes{job="kubelet",metrics_path="/metrics"} > 0 and predict_linear(kubelet_volume_stats_available_bytes{job="kubelet",metrics_path="/metrics"}[6h], 4 * 24 * 3600) < 0  unless on(namespace, persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode="ReadOnlyMany"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{label_excluded_from_alerts="true"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, "name", "$1", "namespace","(vshn-postgresql-test)")`

	patroniMemoryCritical = `label_replace( topk(1, (max(container_memory_working_set_bytes{container="patroni"})without (name, id)  / on(container,pod,namespace)  kube_pod_container_resource_limits{resource="memory"}* 100) > 85) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, "name", "$1", "namespace","(vshn-postgresql-test)")`

	patroniPersistentVolumeFillingUp = `label_replace( bottomk(1, (kubelet_volume_stats_available_bytes{job="kubelet", metrics_path="/metrics"} / kubelet_volume_stats_capacity_bytes{job="kubelet",metrics_path="/metrics"}) < 0.03 and kubelet_volume_stats_used_bytes{job="kubelet",metrics_path="/metrics"} > 0 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode="ReadOnlyMany"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{label_excluded_from_alerts="true"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, "name", "$1", "namespace","(vshn-postgresql-test)")`

	mariadbPersistentVolumeFillingUp = `label_replace( bottomk(1, (kubelet_volume_stats_available_bytes{job="kubelet", metrics_path="/metrics"} / kubelet_volume_stats_capacity_bytes{job="kubelet",metrics_path="/metrics"}) < 0.03 and kubelet_volume_stats_used_bytes{job="kubelet",metrics_path="/metrics"} > 0 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode="ReadOnlyMany"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{label_excluded_from_alerts="true"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, "name", "$1", "namespace","(vshn-mariadb-myinstance)")`

	keycloakMemoryCritical = `label_replace( topk(1, (max(container_memory_working_set_bytes{container="keycloak"})without (name, id)  / on(container,pod,namespace)  kube_pod_container_resource_limits{resource="memory"}* 100) > 85) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, "name", "$1", "namespace","(vshn-keycloak-myinstance)")`
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
	expectedAlerts := []alert{pvFillUp, pvExpectedFillUp, memCritical}

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

	}
	// working with arrays is not the best way, but it's enough for this test
	assert.Equal(t, 3, checkCount)

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
