package vshnredis

import (
	"fmt"
	"maps"

	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const loadBalancerIPConnectionDetailsField = "LOADBALANCER_IP"

func addLoadbalancerConfig(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNRedis, values map[string]any) error {
	if comp.Spec.Parameters.Network.ServiceType != string(corev1.ServiceTypeLoadBalancer) {
		return nil
	}

	if !common.ExternalAccessEnabled(svc) {
		svc.AddResult(runtime.NewWarningResult("LoadBalancer requested but external connections are not enabled"))
		return nil
	}

	serviceConfig := map[string]any{
		"type": string(corev1.ServiceTypeLoadBalancer),
	}

	if ipFilter := comp.Spec.Parameters.Network.IPFilter; len(ipFilter) > 0 {
		ranges := make([]any, 0, len(ipFilter))
		for _, r := range ipFilter {
			ranges = append(ranges, r)
		}
		serviceConfig["loadBalancerSourceRanges"] = ranges
	}

	if svc.Config.Data["loadbalancerAnnotations"] != "" {
		annotations := map[string]string{}
		if err := yaml.Unmarshal([]byte(svc.Config.Data["loadbalancerAnnotations"]), &annotations); err != nil {
			svc.Log.Error(err, "cannot unmarshal loadbalancer annotations from input")
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot unmarshal loadbalancer annotations from input: %s", err)))
		} else {
			serviceConfig["annotations"] = annotations
		}
	}

	if err := common.AddLoadbalancerNetpolicy(svc, comp); err != nil {
		return err
	}

	mergeChartServiceConfig(values, loadBalancerServiceValuePath(comp), serviceConfig)
	return nil
}

func loadBalancerServiceValuePath(comp *vshnv1.VSHNRedis) []string {
	if comp.GetInstances() > 1 {
		return []string{"sentinel", "masterService"}
	}
	return []string{"sentinel", "service"}
}

func loadBalancerServiceName(comp *vshnv1.VSHNRedis) string {
	if comp.GetInstances() > 1 {
		return redisMasterServiceName
	}
	return "redis"
}

// mergeChartServiceConfig deep-merges the given leaf values into
// values[path...], creating intermediate maps as needed and preserving any
// sibling keys already present.
func mergeChartServiceConfig(values map[string]any, path []string, cfg map[string]any) {
	current := values
	for _, k := range path {
		next, ok := current[k].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[k] = next
		}
		current = next
	}
	maps.Copy(current, cfg)
}

func loadBalancerConnectionDetail(comp *vshnv1.VSHNRedis) xhelmv1.ConnectionDetail {
	return xhelmv1.ConnectionDetail{
		ObjectReference: corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Service",
			Name:       loadBalancerServiceName(comp),
			Namespace:  comp.GetInstanceNamespace(),
			FieldPath:  "status.loadBalancer.ingress[0].ip",
		},
		ToConnectionSecretKey:  loadBalancerIPConnectionDetailsField,
		SkipPartOfReleaseCheck: true,
	}
}
