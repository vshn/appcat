package common

import (
	"context"
	"fmt"
	"strings"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	promV1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	synTeam          string = "schedar"
	severityCritical string = "critical"
	memoryContainers        = map[string]string{
		"mariadb":    "mariadb",
		"minio":      "minio",
		"postgresql": "patroni",
		"redis":      "redis",
		"keycloak":   "keycloak",
	}
	minuteInterval, hourInterval, twoHourInterval promV1.Duration = "1m", "1h", "2h"
)

type AlertsEnabled struct {
	All           bool
	DiskFillingUp bool
	DiskSpace     bool
	Memory        bool
}

type RuleStruct struct {
	name, serviceName, instanceNamespace string
	instanceNamespaceRegex               string
	ruleList                             []promV1.Rule
}

func GenerateNonSLAPromRules(obj client.Object, alerts AlertsEnabled) func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {
	return func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {

		log := controllerruntime.LoggerFrom(ctx)
		log.Info("Starting non SLA prometheus rules")

		log.V(1).Info("Transforming", "obj", svc)

		err := svc.GetObservedComposite(obj)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
		}
		elem, ok := obj.(InfoGetter)
		if !ok {
			return runtime.NewFatalResult(err)
		}

		if alerts.All {
			alerts.DiskFillingUp = true
			alerts.DiskSpace = true
			alerts.Memory = true
		}

		ruleObj := &RuleStruct{}
		ruleObj.Configure(elem)

		if alerts.DiskFillingUp {
			ruleObj.WithDiskFillingUP()
		}

		if alerts.DiskSpace {
			ruleObj.WithDisk()
		}

		if alerts.Memory {
			ruleObj.WithMemory()
		}

		err = generatePromeRules(ruleObj, svc)
		if err != nil {
			log.Info("broken addition")
			return runtime.NewWarningResult("can't create prometheus rules: " + err.Error())
		}

		return nil
	}
}

func generatePromeRules(ruleObj *RuleStruct, svc *runtime.ServiceRuntime) error {

	prometheusRules := &promV1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleObj.serviceName + "-non-slo-rules",
			Namespace: ruleObj.instanceNamespace,
		},
		Spec: promV1.PrometheusRuleSpec{
			Groups: []promV1.RuleGroup{
				{
					Name:  ruleObj.serviceName + "-non-slo-rules",
					Rules: ruleObj.ruleList,
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(prometheusRules, ruleObj.name)
}

func (r *RuleStruct) Configure(infogetter InfoGetter) *RuleStruct {
	r.instanceNamespace = infogetter.GetInstanceNamespace()
	r.serviceName = infogetter.GetName()

	instanceNamespaceRegex, instanceNamespaceSplitted, err := getInstanceNamespaceRegex(r.instanceNamespace)
	if err != nil {
		controllerruntime.LoggerFrom(context.Background()).Error(err, "getInstanceNamespaceRegex func failed to parse instance namespace")
		return nil
	}

	r.instanceNamespaceRegex = instanceNamespaceRegex
	r.serviceName = memoryContainers[instanceNamespaceSplitted[1]]
	r.name = infogetter.GetName() + "-non-sla-alerts"

	return r
}

func (r *RuleStruct) WithDiskFillingUP() *RuleStruct {
	r.ruleList = append(r.ruleList, promV1.Rule{
		Alert: r.serviceName + "PersistentVolumeFillingUp",
		Annotations: map[string]string{
			"description": "The volume claimed by the instance {{ $labels.name }} in namespace {{ $labels.label_appcat_vshn_io_claim_namespace }} is only {{ $value | humanizePercentage }} free.",
			"runbook_url": "https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepersistentvolumefillingup",
			"summary":     "PersistentVolume is filling up.",
		},
		Expr: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: fmt.Sprintf("label_replace( bottomk(1, (kubelet_volume_stats_available_bytes{job=\"kubelet\", metrics_path=\"/metrics\"} / kubelet_volume_stats_capacity_bytes{job=\"kubelet\",metrics_path=\"/metrics\"}) < 0.03 and kubelet_volume_stats_used_bytes{job=\"kubelet\",metrics_path=\"/metrics\"} > 0 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode=\"ReadOnlyMany\"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{label_excluded_from_alerts=\"true\"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, \"name\", \"$1\", \"namespace\",\"%s\")", r.instanceNamespaceRegex),
		},
		For: minuteInterval,
		Labels: map[string]string{
			"severity": severityCritical,
			"syn_team": synTeam,
		},
	})
	return r
}

func (r *RuleStruct) WithDisk() *RuleStruct {
	r.ruleList = append(r.ruleList, promV1.Rule{
		Alert: r.serviceName + "PersistentVolumeExpectedToFillUp",
		Annotations: map[string]string{
			"description": "Based on recent sampling, the volume claimed by the instance {{ $labels.name }} in namespace {{ $labels.label_appcat_vshn_io_claim_namespace }} is expected to fill up within four days. Currently {{ $value | humanizePercentage }} is available.",
			"runbook_url": "https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepersistentvolumefillingup",
			"summary":     "PersistentVolume is expected to fill up.",
		},
		Expr: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: fmt.Sprintf("label_replace( bottomk(1, (kubelet_volume_stats_available_bytes{job=\"kubelet\",metrics_path=\"/metrics\"} / kubelet_volume_stats_capacity_bytes{job=\"kubelet\",metrics_path=\"/metrics\"}) < 0.15 and kubelet_volume_stats_used_bytes{job=\"kubelet\",metrics_path=\"/metrics\"} > 0 and predict_linear(kubelet_volume_stats_available_bytes{job=\"kubelet\",metrics_path=\"/metrics\"}[6h], 4 * 24 * 3600) < 0  unless on(namespace, persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode=\"ReadOnlyMany\"} == 1 unless on(namespace,persistentvolumeclaim) kube_persistentvolumeclaim_labels{label_excluded_from_alerts=\"true\"}== 1) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, \"name\", \"$1\", \"namespace\",\"%s\")", r.instanceNamespaceRegex),
		},
		For: hourInterval,
		Labels: map[string]string{
			"severity": severityCritical,
			"syn_team": synTeam,
		},
	})
	return r
}

func (r *RuleStruct) WithMemory() *RuleStruct {
	r.ruleList = append(r.ruleList, promV1.Rule{
		Alert: r.serviceName + "MemoryCritical",
		Annotations: map[string]string{
			"description": "The memory claimed by the instance {{ $labels.name }} in namespace {{ $labels.label_appcat_vshn_io_claim_namespace }} has been over 85% for 2 hours.\n  Please reduce the load of this instance, or increase the memory.",
			"runbook_url": "https://hub.syn.tools/appcat/runbooks/vshn-generic.html#MemoryCritical",
			"summary":     "Memory usage critical.",
		},
		Expr: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: fmt.Sprintf("label_replace( topk(1, (max(container_memory_working_set_bytes{container=\"%s\"})without (name, id)  / on(container,pod,namespace)  kube_pod_container_resource_limits{resource=\"memory\"}* 100) > 85) * on(namespace) group_left(label_appcat_vshn_io_claim_namespace)kube_namespace_labels, \"name\", \"$1\", \"namespace\",\"%s\")", r.serviceName, r.instanceNamespaceRegex),
		},
		For: twoHourInterval,
		Labels: map[string]string{
			"severity": severityCritical,
			"syn_team": synTeam,
		},
	})
	return r
}

// Get InstanceNamespaceRegex returns regex for prometheus rules, splitted instance namespace and error if necessary
func getInstanceNamespaceRegex(instanceNamespace string) (string, []string, error) {
	// from instance namespace, f.e. vshn-postgresql-customer-namespace-whatever
	// make vshn-postgresql-(.+)-.+
	//		vshn-redis-(.+)-.+
	//		vshn-minio-(.+)-.+
	// required for Prometheus queries

	// vshn- <- takes 5 letters, anything shorter that 7 makes no sense
	if len(instanceNamespace) < 7 {
		return "", nil, fmt.Errorf("GetInstanceNamespaceRegex: instance namespace is way too short")
	}

	splitted := strings.Split(instanceNamespace, "-")
	// at least [vshn, serviceName] should be present
	if len(splitted) < 3 {
		return "", nil, fmt.Errorf("GetInstanceNamespaceRegex: instance namespace broken during splitting")
	}

	for _, val := range splitted {
		if len(val) == 0 {
			return "", nil, fmt.Errorf("GetInstanceNamespaceRegex: broken instance namespace, name ending with hyphen: %s", val)
		}
	}

	return fmt.Sprintf("%s-%s-(.+)-.+", splitted[0], splitted[1]), splitted, nil
}
