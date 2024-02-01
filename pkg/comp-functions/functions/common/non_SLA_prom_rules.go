package common

import (
	"context"
	"fmt"
	"reflect"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	promV1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	SYN_TEAM          string = "schedar"
	SEVERITY_CRITICAL string = "critical"
	MEMORY_CONTAINERS        = map[string]string{
		"mariadb":    "mariadb",
		"minio":      "minio",
		"postgresql": "patroni",
		"redis":      "redis",
	}
)

func GenerateNonSLAPromRules(obj client.Object) func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {
	return func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {

		log := controllerruntime.LoggerFrom(ctx)
		log.Info("adding non SLA prometheus rules")

		log.V(1).Info("Transforming", "obj", svc)

		err := svc.GetObservedComposite(obj)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("Can't get composite: %w", err))
		}
		elem, ok := obj.(InfoGetter)
		if !ok {
			return runtime.NewWarningResult(fmt.Sprintf("Type %s doesn't implement Alerter interface", reflect.TypeOf(obj).String()))
		}

		instanceNamespaceRegex, instanceNamespaceSplitted, err := elem.GetInstanceNamespaceRegex()
		if err != nil {
			return runtime.NewWarningResult("Instance namespace looks broken, " + err.Error())
		}

		err = generatePromeRules(elem.GetName(), elem.GetInstanceNamespace(), instanceNamespaceRegex, MEMORY_CONTAINERS[instanceNamespaceSplitted[1]], svc)
		if err != nil {
			return runtime.NewWarningResult("can't create prometheus rules: " + err.Error())
		}

		return nil
	}
}

func generatePromeRules(name, namespace, namespaceRegex, serviceName string, svc *runtime.ServiceRuntime) error {
	var minuteInterval, hourInterval, twoHourInterval promV1.Duration
	minuteInterval = "1m"
	hourInterval = "1h"
	twoHourInterval = "2h"

	prometheusRules := &promV1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-non-slo-rules",
			Namespace: namespace,
		},
		Spec: promV1.PrometheusRuleSpec{
			Groups: []promV1.RuleGroup{
				promV1.RuleGroup{
					Name: serviceName + "-non-slo-rules",
					Rules: []promV1.Rule{
						promV1.Rule{
							Alert: serviceName + "PersistentVolumeFillingUp",
							Annotations: map[string]string{
								"description": "The volume claimed by the instance {{ $labels.name }} in namespace {{ $labels.label_appcat_vshn_io_claim_namespace }} is only {{ $value | humanizePercentage }} free.",
								"runbook_url": "https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepersistentvolumefillingup",
								"summary":     "PersistentVolume is filling up.",
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("label_replace( bottomk(1, (kubelet_volume_stats_available_bytes{job=\"kubelet\", metrics_path=\"/metrics\"} / kubelet_volume_stats_capacity_bytes{job=\"kubelet\",metrics_path=\"/metrics\"}) < 0.03 and kubelet_volume_stats_used_bytes{job=\"kubelet\",metrics_path=\"/metrics\"} > 0 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode=\"ReadOnlyMany\"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{label_excluded_from_alerts=\"true\"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, \"name\", \"$1\", \"namespace\",\"%s\")", namespaceRegex),
							},
							For: minuteInterval,
							Labels: map[string]string{
								"severity": SEVERITY_CRITICAL,
								"syn_team": SYN_TEAM,
							},
						},
						promV1.Rule{
							Alert: serviceName + "PersistentVolumeExpectedToFillUp",
							Annotations: map[string]string{
								"description": "Based on recent sampling, the volume claimed by the instance {{ $labels.name }} in namespace {{ $labels.label_appcat_vshn_io_claim_namespace }} is expected to fill up within four days. Currently {{ $value | humanizePercentage }} is available.",
								"runbook_url": "https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepersistentvolumefillingup",
								"summary":     "PersistentVolume is expected to fill up.",
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("label_replace( bottomk(1, (kubelet_volume_stats_available_bytes{job=\"kubelet\",metrics_path=\"/metrics\"} / kubelet_volume_stats_capacity_bytes{job=\"kubelet\",metrics_path=\"/metrics\"}) < 0.15 and kubelet_volume_stats_used_bytes{job=\"kubelet\",metrics_path=\"/metrics\"} > 0 and predict_linear(kubelet_volume_stats_available_bytes{job=\"kubelet\",metrics_path=\"/metrics\"}[6h], 4 * 24 * 3600) < 0  unless on(namespace, persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode=\"ReadOnlyMany\"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{label_excluded_from_alerts=\"true\"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, \"name\", \"$1\", \"namespace\",\"%s\")", namespaceRegex),
							},
							For: hourInterval,
							Labels: map[string]string{
								"severity": SEVERITY_CRITICAL,
								"syn_team": SYN_TEAM,
							},
						},
						promV1.Rule{
							Alert: serviceName + "MemoryCritical",
							Annotations: map[string]string{
								"description": "The memory claimed by the instance {{ $labels.name }} in namespace {{ $labels.label_appcat_vshn_io_claim_namespace }} has been over 85% for 2 hours.\n  Please reducde the load of this instance, or increase the memory.",
								"runbook_url": "https://hub.syn.tools/appcat/runbooks/vshn-generic.html#MemoryCritical",
								"summary":     "Memory usage critical.",
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("label_replace( topk(1, (max(container_memory_working_set_bytes{container=\"%s\"})without (name, id)  / on(container,pod,namespace)  kube_pod_container_resource_limits{resource=\"memory\"}* 100) > 85) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, \"name\", \"$1\", \"namespace\",\"%s\")", serviceName, namespaceRegex),
							},
							For: twoHourInterval,
							Labels: map[string]string{
								"severity": SEVERITY_CRITICAL,
								"syn_team": SYN_TEAM,
							},
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(prometheusRules, name+"-non-sla-alerts")
}
