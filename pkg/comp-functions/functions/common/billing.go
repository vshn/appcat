package common

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	v12 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"strings"
	"text/template"
)

var rawExpr = "sum (sum_over_time ((kube_namespace_labels{namespace=\"{{.Namespace}}\"} * {{.Instances}})[60m:1m])/60)"

// Billing contains required information for the billing record expression
type Billing struct {
	Namespace string
	Instances int
}

const billingLabel = "appcat.io/billing"

// TODO remove this function as soon as we have automated release pipeline in prod according to maintenance
// InjectBillingLabelToService adds billing label to a service (StatefulSet or Deployment).
// It uses a kube Object to achieve post provisioning labelling
func InjectBillingLabelToService(ctx context.Context, svc *runtime.ServiceRuntime, comp InfoGetter) *xfnproto.Result {

	s := comp.GetWorkloadPodTemplateLabelsManager()
	s.SetName(comp.GetWorkloadName())
	s.SetNamespace(comp.GetInstanceNamespace())
	kubeName := comp.GetName() + "-" + getType(s)

	_ = svc.GetObservedKubeObject(s, kubeName)
	mp := v12.ManagementPolicies{v12.ManagementActionObserve}
	labels := s.GetPodTemplateLabels()
	_, exists := labels[billingLabel]
	if !s.GetCreationTimestamp().Time.IsZero() {
		if !exists {
			labels[billingLabel] = "true"
			s.SetPodTemplateLabels(labels)
			mp = append(mp, v12.ManagementActionCreate, v12.ManagementActionUpdate)
		}
	}

	err := svc.SetDesiredKubeObject(s.GetObject(), kubeName, func(obj *xkube.Object) {
		obj.Spec.ManagementPolicies = mp
	})

	if err != nil && !exists {
		runtime.NewWarningResult(fmt.Sprintf("cannot add billing to service object %s", s.GetName()))
	}

	return runtime.NewNormalResult("billing enabled")
}

func getType(myvar interface{}) (res string) {
	return strings.ToLower(reflect.TypeOf(myvar).Elem().Field(0).Name)
}

// CreateBillingRecord creates a new prometheus rule per each instance namespace
// The rule is skipped for any secondary service such as postgresql instance for nextcloud
// The skipping is based on whether label appuio.io/billing-name is set or not on instance namespace
func CreateBillingRecord(ctx context.Context, svc *runtime.ServiceRuntime, comp InfoGetter) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Enabling billing for service", "service", comp.GetName())

	expr, err := getExprFromTemplate(comp.GetInstanceNamespace(), comp.GetInstances())
	if err != nil {
		runtime.NewWarningResult(fmt.Sprintf("cannot add billing to service %s", comp.GetName()))
	}

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

	p := &v1.PrometheusRule{
		Spec: v1.PrometheusRuleSpec{
			Groups: []v1.RuleGroup{
				{
					Name: "appcat-metering-rules",
					Rules: []v1.Rule{
						{
							Record: "appcat:metering",
							Expr:   intstr.FromString(expr),
							Labels: getLabels(org, comp, svc),
						},
					},
				},
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

func getLabels(org string, comp InfoGetter, svc *runtime.ServiceRuntime) map[string]string {
	labels := map[string]string{
		"label_appcat_vshn_io_claim_name":      comp.GetClaimName(),
		"label_appcat_vshn_io_claim_namespace": comp.GetClaimNamespace(),
		"label_appcat_vshn_io_sla":             comp.GetSLA(),
		"label_appuio_io_billing_name":         comp.GetBillingName(),
		"label_appuio_io_organization":         org,
	}

	so := svc.Config.Data["salesOrder"]
	// if appuio managed then add the sales order
	if so != "" {
		labels["sales_order"] = so
	}
	return labels
}

func getExprFromTemplate(namespace string, i int) (string, error) {
	b := Billing{
		Instances: i,
		Namespace: namespace,
	}

	var buf bytes.Buffer
	tmpl, err := template.New("billing").Parse(rawExpr)
	if err != nil {
		return "", err
	}

	err = tmpl.Execute(&buf, b)
	if err != nil {
		return "", err
	}

	return buf.String(), err
}
