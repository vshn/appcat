package common

import (
	"context"
	_ "embed"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

// ServiceAddOns describes an addOn for a services with necessary data for billing
type ServiceAddOns struct {
	Name      string
	Instances int
}

// CreateBillingRecord creates a new prometheus rule per each instance namespace
// The rule is skipped for any secondary service such as postgresql instance for nextcloud
// The skipping is based on whether label appuio.io/billing-name is set or not on instance namespace
func CreateBillingRecord(ctx context.Context, svc *runtime.ServiceRuntime, comp InfoGetter, addOns ...ServiceAddOns) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Enabling billing for service", "service", comp.GetName())

	if comp.GetClaimNamespace() == svc.Config.Data["ignoreNamespaceForBilling"] {
		log.Info("Test instance, skipping billing")
		return nil
	}

	expr := getVectorExpression(comp.GetInstances())

	org, err := getOrg(comp.GetName(), svc)
	if err != nil {
		log.Error(err, "billing not working, cannot get organization", "service", comp.GetName())
		return runtime.NewWarningResult(fmt.Sprintf("cannot add billing to service %s", comp.GetName()))
	}

	controlNS, ok := svc.Config.Data["controlNamespace"]
	if !ok {
		log.Error(err, "billing not working, control namespace missing", "service", comp.GetName())
		return runtime.NewWarningResult(fmt.Sprintf("cannot add billing to service %s", comp.GetName()))
	}

	disabled, err := isBillingDisabled(controlNS, comp.GetInstanceNamespace(), comp.GetName(), svc)
	if err != nil {
		log.Error(err, "billing not working, cannot determine if primary service", "service", comp.GetName())
		return runtime.NewWarningResult(fmt.Sprintf("cannot add billing to service %s", comp.GetName()))
	}

	if disabled {
		log.Info("secondary service, skipping billing", "service", comp.GetName())
		return runtime.NewNormalResult(fmt.Sprintf("billing disabled for instance %s", comp.GetName()))
	}
	rg := v1.RuleGroup{
		Name: "appcat-metering-rules",
		Rules: []v1.Rule{
			{
				Record: "appcat:metering",
				Expr:   intstr.FromString(expr),
				Labels: getLabels(svc, comp, org, ""),
			},
		},
	}

	for _, addOn := range addOns {
		log.Info("Adding billing addOn for service", "service", comp.GetName(), "addOn", addOn.Name)
		exprAddOn := getVectorExpression(addOn.Instances)
		rg.Rules = append(rg.Rules, v1.Rule{
			Record: "appcat:metering",
			Expr:   intstr.FromString(exprAddOn),
			Labels: getLabels(svc, comp, org, addOn.Name),
		})
	}

	p := &v1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				// This label is required, the ensure that the rule is added to Prometheus and
				// not the Thanos Rules in the Openshift User Workload Monitoring
				"openshift.io/prometheus-rule-evaluation-scope": "leaf-prometheus",
			},
		},
		Spec: v1.PrometheusRuleSpec{
			Groups: []v1.RuleGroup{
				rg,
			},
		},
	}
	p.SetName(comp.GetName() + "-billing")
	p.SetNamespace(comp.GetInstanceNamespace())
	kubeName := comp.GetName() + "-billing"

	err = svc.SetDesiredKubeObject(p, kubeName)

	if err != nil {
		log.Error(err, "cannot add billing to service, cannot set desired object", "service", comp.GetName())
		return runtime.NewWarningResult(fmt.Sprintf("cannot add billing to service %s", p.GetName()))
	}

	return runtime.NewNormalResult(fmt.Sprintf("billing enabled for instance %s", comp.GetName()))
}

func getLabels(svc *runtime.ServiceRuntime, comp InfoGetter, org, addOnName string) map[string]string {
	b, err := getBillingNameWithAddOn(comp.GetBillingName(), addOnName)
	if err != nil {
		panic(fmt.Errorf("set billing name for service %s: %v", comp.GetServiceName(), err))
	}
	labels := map[string]string{
		"label_appcat_vshn_io_claim_name":      comp.GetClaimName(),
		"label_appcat_vshn_io_claim_namespace": comp.GetClaimNamespace(),
		"label_appcat_vshn_io_sla":             comp.GetSLA(),
		"label_appcat_vshn_io_addon_name":      addOnName,
		"label_appuio_io_billing_name":         b,
		"label_appuio_io_organization":         org,
	}

	so := svc.Config.Data["salesOrder"]
	// if appuio managed then add the sales order
	if so != "" {
		labels["sales_order"] = so
	}
	return labels
}

func getBillingNameWithAddOn(billingName, addOn string) (string, error) {
	if billingName == "" {
		return "", fmt.Errorf("billing name is empty")
	}
	if addOn == "" {
		return billingName, nil
	}
	return fmt.Sprintf("%s-%s", billingName, addOn), nil
}

func getVectorExpression(i int) string {
	return fmt.Sprintf("vector(%d)", i)
}
