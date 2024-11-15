package nonsla

import (
	promV1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServiceRule is a func definition to get a specific rule based on a container name s and namespace name n
type ServiceRule func(s, n string) promV1.Rule

// alert non-exportable alert type to be used only in this package
type alert string

// Alerts the object that the service must create to generate non sla prometheus rules
type Alerts struct {
	// customRules are a set of rules specific to a services
	customRules []promV1.Rule
	// alerts are generic alerts defined for all services of appcat
	alerts []alert
	// alertContainerName is the container name to be used for alert names alert expression
	alertContainerName string

	// Alertdefinitions contains all the ServiceRules
	alertDefinitions map[alert]ServiceRule
}

const (
	SynTeam                                       string          = "schedar"
	SeverityCritical                              string          = "critical"
	MinuteInterval, HourInterval, TwoHourInterval promV1.Duration = "1m", "1h", "2h"
)

var (
	// alertDefinitions is a map of alert definitions which has the name of alerts as key and the func ServiceRule as value
	alertDefinitions = map[alert]ServiceRule{

		pvFillUp: func(name, namespace string) promV1.Rule {
			return promV1.Rule{
				Alert: "PersistentVolumeFillingUp",
				Annotations: map[string]string{
					"description": "The volume claimed by the instance {{ $labels.name }} in namespace {{ $labels.label_appcat_vshn_io_claim_namespace }} is only {{ $value | humanizePercentage }} free.",
					"runbook_url": "https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepersistentvolumefillingup",
					"summary":     "PersistentVolume is filling up.",
				},
				Expr: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "label_replace( bottomk(1, (kubelet_volume_stats_available_bytes{job=\"kubelet\", metrics_path=\"/metrics\"} / kubelet_volume_stats_capacity_bytes{job=\"kubelet\",metrics_path=\"/metrics\"}) < 0.03 and kubelet_volume_stats_used_bytes{job=\"kubelet\",metrics_path=\"/metrics\"} > 0 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode=\"ReadOnlyMany\"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{label_excluded_from_alerts=\"true\"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, \"name\", \"$1\", \"namespace\",\"" + namespace + "\")",
				},
				For: MinuteInterval,
				Labels: map[string]string{
					"severity": SeverityCritical,
					"syn_team": SynTeam,
					"syn":      "true",
				},
			}
		},
		pvExpectedFillUp: func(name, namespace string) promV1.Rule {
			return promV1.Rule{
				Alert: "PersistentVolumeExpectedToFillUp",
				Annotations: map[string]string{
					"description": "Based on recent sampling, the volume claimed by the instance {{ $labels.name }} in namespace {{ $labels.label_appcat_vshn_io_claim_namespace }} is expected to fill up within four days. Currently {{ $value | humanizePercentage }} is available.",
					"runbook_url": "https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepersistentvolumefillingup",
					"summary":     "PersistentVolume is expected to fill up.",
				},
				Expr: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "label_replace( bottomk(1, (kubelet_volume_stats_available_bytes{job=\"kubelet\",metrics_path=\"/metrics\"} / kubelet_volume_stats_capacity_bytes{job=\"kubelet\",metrics_path=\"/metrics\"}) < 0.15 and kubelet_volume_stats_used_bytes{job=\"kubelet\",metrics_path=\"/metrics\"} > 0 and predict_linear(kubelet_volume_stats_available_bytes{job=\"kubelet\",metrics_path=\"/metrics\"}[6h], 4 * 24 * 3600) < 0  unless on(namespace, persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode=\"ReadOnlyMany\"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{label_excluded_from_alerts=\"true\"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, \"name\", \"$1\", \"namespace\",\"" + namespace + "\")",
				},
				For: HourInterval,
				Labels: map[string]string{
					"severity": SeverityCritical,
					"syn_team": SynTeam,
					"syn":      "true",
				},
			}
		},
		memCritical: func(name, namespace string) promV1.Rule {
			return promV1.Rule{
				Alert: "MemoryCritical",
				Annotations: map[string]string{
					"description": "The memory claimed by the instance {{ $labels.name }} in namespace {{ $labels.label_appcat_vshn_io_claim_namespace }} has been over 85% for 2 hours.\n  Please reduce the load of this instance, or increase the memory.",
					"runbook_url": "https://hub.syn.tools/appcat/runbooks/vshn-generic.html#MemoryCritical",
					"summary":     "Memory usage critical.",
				},
				Expr: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "label_replace( topk(1, (max(container_memory_working_set_bytes{container=\"" + name + "\"})without (name, id)  / on(container,pod,namespace)  kube_pod_container_resource_limits{resource=\"memory\"}* 100) > 85) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, \"name\", \"$1\", \"namespace\",\"" + namespace + "\")",
				},
				For: TwoHourInterval,
				Labels: map[string]string{
					"severity": SeverityCritical,
					"syn_team": SynTeam,
					"syn":      "true",
				},
			}
		},
	}
)

var (
	pvFillUp         alert = "PersistentVolumeFillingUp"
	pvExpectedFillUp alert = "PersistentVolumeExpectedToFillUp"
	memCritical      alert = "MemoryCritical"
)

type AlertBuilder struct {
	as Alerts
}

func NewAlertSetBuilder(containerName string) *AlertBuilder {
	return &AlertBuilder{as: Alerts{
		customRules:        make([]promV1.Rule, 0),
		alerts:             make([]alert, 0),
		alertContainerName: containerName,
		alertDefinitions:   alertDefinitions,
	}}
}

func (a *AlertBuilder) AddDiskFillingUp() *AlertBuilder {
	a.as.alerts = append(a.as.alerts, pvFillUp)
	return a
}

func (a *AlertBuilder) AddDisk() *AlertBuilder {
	a.as.alerts = append(a.as.alerts, pvExpectedFillUp)
	return a
}

func (a *AlertBuilder) AddMemory() *AlertBuilder {
	a.as.alerts = append(a.as.alerts, memCritical)
	return a
}

func (a *AlertBuilder) AddCustom(r []promV1.Rule) *AlertBuilder {
	a.as.customRules = r
	return a
}

func (a *AlertBuilder) AddCustomServiceRule(name string, rule ServiceRule) *AlertBuilder {
	a.as.alerts = append(a.as.alerts, alert(name))
	a.as.alertDefinitions[alert(name)] = rule
	return a
}

func (a *AlertBuilder) AddAll() *AlertBuilder {
	a.as.alerts = make([]alert, 0)
	for alert, _ := range a.as.alertDefinitions {
		a.as.alerts = append(a.as.alerts, alert)
	}
	return a
}

func (a *AlertBuilder) GetAlerts() Alerts {
	return a.as
}
