package common

import (
	"context"
	_ "embed"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	v2 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"
)

func TestBilling(t *testing.T) {

	tests := []struct {
		name              string
		comp              *v1.VSHNNextcloud
		svc               *runtime.ServiceRuntime
		org               string
		addOns            []ServiceAddOns
		expectedRuleGroup v2.RuleGroup
	}{
		{
			name: "TestCreateBillingRecord_WhenServiceWithAddOn_ThenReturnMultipleRules",
			comp: &v1.VSHNNextcloud{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"crossplane.io/claim-namespace": "test-namespace",
						"crossplane.io/claim-name":      "test-instance",
					},
					Name:      "nextcloud-gc9x4",
					Namespace: "unit-test",
				},
				Spec: v1.VSHNNextcloudSpec{
					Parameters: v1.VSHNNextcloudParameters{
						Instances: 1,
						Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
					},
				},
			},
			svc: commontest.LoadRuntimeFromFile(t, "common/billing.yaml"),
			org: "APPUiO",
			addOns: []ServiceAddOns{
				{
					Name:      "office",
					Instances: 1,
				},
			},
			expectedRuleGroup: v2.RuleGroup{
				Name: "appcat-metering-rules",
				Rules: []v2.Rule{
					{
						Record: "appcat:metering",
						Expr: intstr.IntOrString{
							Type:   1,
							StrVal: "vector(1)",
						},
						Labels: map[string]string{
							"label_appcat_vshn_io_claim_name":      "test-instance",
							"label_appcat_vshn_io_claim_namespace": "test-namespace",
							"label_appcat_vshn_io_sla":             "besteffort",
							"label_appcat_vshn_io_addon_name":      "",
							"label_appuio_io_billing_name":         "appcat-nextcloud",
							"label_appuio_io_organization":         "vshn",
						},
					},
					{
						Record: "appcat:metering",
						Expr: intstr.IntOrString{
							Type:   1,
							StrVal: "vector(1)",
						},
						Labels: map[string]string{
							"label_appcat_vshn_io_claim_name":      "test-instance",
							"label_appcat_vshn_io_claim_namespace": "test-namespace",
							"label_appcat_vshn_io_sla":             "besteffort",
							"label_appcat_vshn_io_addon_name":      "office",
							"label_appuio_io_billing_name":         "appcat-nextcloud-office",
							"label_appuio_io_organization":         "vshn",
						},
					},
				},
			},
		},
		{
			name: "TestCreateBillingRecord_WhenServiceWithoutAddOn_ThenReturnSingleRule",
			comp: &v1.VSHNNextcloud{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"crossplane.io/claim-namespace": "test-namespace",
						"crossplane.io/claim-name":      "test-instance",
					},
					Name:      "nextcloud-gc9x4",
					Namespace: "unit-test",
				},
				Spec: v1.VSHNNextcloudSpec{
					Parameters: v1.VSHNNextcloudParameters{
						Instances: 1,
						Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
					},
				},
			},
			svc:    commontest.LoadRuntimeFromFile(t, "common/billing.yaml"),
			org:    "APPUiO",
			addOns: []ServiceAddOns{},
			expectedRuleGroup: v2.RuleGroup{
				Name: "appcat-metering-rules",
				Rules: []v2.Rule{
					{
						Record: "appcat:metering",
						Expr: intstr.IntOrString{
							Type:   1,
							StrVal: "vector(1)",
						},
						Labels: map[string]string{
							"label_appcat_vshn_io_claim_name":      "test-instance",
							"label_appcat_vshn_io_claim_namespace": "test-namespace",
							"label_appcat_vshn_io_sla":             "besteffort",
							"label_appcat_vshn_io_addon_name":      "",
							"label_appuio_io_billing_name":         "appcat-nextcloud",
							"label_appuio_io_organization":         "vshn",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CreateBillingRecord(context.Background(), tt.svc, tt.comp, tt.addOns...)
			assert.Equal(t, xfnproto.Severity_SEVERITY_NORMAL, r.Severity)
			actual := &v2.PrometheusRule{}
			err := tt.svc.GetDesiredKubeObject(actual, "nextcloud-gc9x4-billing")
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedRuleGroup, actual.Spec.Groups[0])

		})
	}
}

func TestGetLabels(t *testing.T) {
	tests := []struct {
		name           string
		comp           *v1.VSHNNextcloud
		svc            *runtime.ServiceRuntime
		org            string
		addOnName      string
		expectedLabels map[string]string
	}{
		{
			name: "TestGetLabels_WhenManagedAndAddOn_ThenReturnLabels",
			comp: &v1.VSHNNextcloud{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"crossplane.io/claim-namespace": "test-namespace",
						"crossplane.io/claim-name":      "test-instance",
					},
				},
				Spec: v1.VSHNNextcloudSpec{
					Parameters: v1.VSHNNextcloudParameters{
						Service: v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
					},
				},
			},
			svc: &runtime.ServiceRuntime{
				Config: api.ConfigMap{
					Data: map[string]string{
						"salesOrder": "12132ST",
					},
				},
			},
			org:       "APPUiO",
			addOnName: "office",
			expectedLabels: map[string]string{
				"label_appcat_vshn_io_claim_name":      "test-instance",
				"label_appcat_vshn_io_claim_namespace": "test-namespace",
				"label_appcat_vshn_io_sla":             "besteffort",
				"label_appcat_vshn_io_addon_name":      "office",
				"label_appuio_io_billing_name":         "appcat-nextcloud-office",
				"label_appuio_io_organization":         "APPUiO",
				"sales_order":                          "12132ST",
			},
		},
		{
			name: "TestGetLabels_WhenNonManagedAndAddOn_ThenReturnLabels",
			comp: &v1.VSHNNextcloud{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"crossplane.io/claim-namespace": "test-namespace",
						"crossplane.io/claim-name":      "test-instance",
					},
				},
				Spec: v1.VSHNNextcloudSpec{
					Parameters: v1.VSHNNextcloudParameters{
						Service: v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
					},
				},
			},
			svc:       &runtime.ServiceRuntime{},
			org:       "APPUiO",
			addOnName: "office",
			expectedLabels: map[string]string{
				"label_appcat_vshn_io_claim_name":      "test-instance",
				"label_appcat_vshn_io_claim_namespace": "test-namespace",
				"label_appcat_vshn_io_sla":             "besteffort",
				"label_appcat_vshn_io_addon_name":      "office",
				"label_appuio_io_billing_name":         "appcat-nextcloud-office",
				"label_appuio_io_organization":         "APPUiO",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := getLabels(tt.svc, tt.comp, tt.org, tt.addOnName)
			assert.Equal(t, tt.expectedLabels, labels)
		})
	}
}

func TestGetBillingNameWithAddOn(t *testing.T) {
	tests := []struct {
		name        string
		billingName string
		addOn       string
		result      string
		err         bool
	}{
		{
			name:        "TestGetBillingNameWithAddOn_WhenAddOn_ThenBillingNameWithAddOn",
			billingName: "nextcloud",
			addOn:       "office",
			result:      "nextcloud-office",
			err:         false,
		},
		{
			name:        "TestGetBillingNameWithAddOn_WhenNoAddOn_ThenJustBillingName",
			billingName: "nextcloud",
			addOn:       "",
			result:      "nextcloud",
			err:         false,
		},
		{
			name:        "TestGetBillingNameWithAddOn_WhenEmpty_ThenError",
			billingName: "",
			addOn:       "",
			result:      "",
			err:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := getBillingNameWithAddOn(tt.billingName, tt.addOn)
			assert.Equal(t, tt.result, actual)
			if tt.err {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestGetExprFromTemplate(t *testing.T) {
	tests := []struct {
		name      string
		instances int
		result    string
	}{
		{
			name:      "TestGetExprFromTemplate_WhenInstances1_ThenExprWithInstance",
			instances: 1,
			result:    "vector(1)",
		},
		{
			name:      "TestGetExprFromTemplate_WhenInstances0_ThenExprWithInstance",
			instances: 0,
			result:    "vector(0)",
		},
		{
			name:      "TestGetExprFromTemplate_WhenInstancesHA_ThenExprWithInstance",
			instances: 3,
			result:    "vector(3)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getVectorExpression(tt.instances)
			assert.Equal(t, tt.result, actual)
		})
	}
}
