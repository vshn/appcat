package nonsla

import (
	"context"
	"fmt"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	promV1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenerateNonSLAPromRules(obj client.Object, alerts Alerts) func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {
	return func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {
		log := controllerruntime.LoggerFrom(ctx)
		log.Info("Starting non SLA prometheus rules")

		log.V(1).Info("Transforming", "obj", svc)

		err := svc.GetObservedComposite(obj)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
		}
		elem, ok := obj.(common.InfoGetter)
		if !ok {
			return runtime.NewFatalResult(err)
		}

		rules := make([]promV1.Rule, 0)
		for _, a := range alerts.alerts {
			f := AlertDefinitions[a]
			r := f(alerts.alertContainerName, alerts.namespace)
			rules = append(rules, r)
		}
		rules = append(rules, alerts.customRules...)

		err = generatePromeRules(elem.GetName(), elem.GetInstanceNamespace(), rules, svc)
		if err != nil {
			return runtime.NewWarningResult("can't create prometheus rules: " + err.Error())
		}

		return nil
	}
}

func generatePromeRules(s, namespace string, rules []promV1.Rule, svc *runtime.ServiceRuntime) error {
	name := s + "-non-slo-rules"
	prometheusRules := &promV1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: promV1.PrometheusRuleSpec{
			Groups: []promV1.RuleGroup{
				{
					Name:  name,
					Rules: rules,
				},
			},
		},
	}
	return svc.SetDesiredKubeObject(prometheusRules, name)
}
