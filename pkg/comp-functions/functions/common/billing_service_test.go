package common

import (
	"context"
	"testing"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBillingService_BillingDisabled(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_disabled.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nc-xyz",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "prod-ns",
				"crossplane.io/claim-name":      "my-nc",
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_NORMAL, result.Severity)
}

func TestBillingService_IgnoredNamespace(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_ignored_ns.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nc-xyz",
			Labels: map[string]string{
				// claim-namespace matches ignoreNamespaceForBilling in the fixture
				"crossplane.io/claim-namespace": "prod-ns",
				"crossplane.io/claim-name":      "my-nc",
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_NORMAL, result.Severity)
}

func TestBillingService_JSONAnnotation_HappyPath(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_annotation.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "codey-abc12",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "test-ns",
				"crossplane.io/claim-name":      "my-codey",
			},
			Annotations: map[string]string{
				"billing.servala.com/salesOrderID": "S12509",
				"billing.servala.com/items":        `{"items":[{"itemDescription":"si-84c17714 on Cloudscale RMA","itemGroupDescription":"Servala Service: Codey","productID":"codey-mini","value":"1"},{"itemDescription":"si-84c17714 on Cloudscale RMA","itemGroupDescription":"Servala Service: Codey","productID":"cloudscale-ssd","value":"10"}]}`,
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_NORMAL, result.Severity)

	bs := &v1.BillingService{}
	err := svc.GetDesiredKubeObject(bs, "codey-abc12-billing-service")
	require.NoError(t, err)

	// salesOrderID annotation overrides config-level salesOrder
	assert.Equal(t, "S12509", bs.Spec.Odoo.SalesOrderID)
	// Organization must not be set (not APPUiO Cloud billing)
	assert.Empty(t, bs.Spec.Odoo.Organization)

	items := bs.Spec.Odoo.Items
	require.Len(t, items, 2)

	assert.Equal(t, "codey-mini", items[0].ProductID)
	assert.Equal(t, "1", items[0].Value)
	assert.Equal(t, "si-84c17714 on Cloudscale RMA", items[0].ItemDescription)
	assert.Equal(t, "Servala Service: Codey", items[0].ItemGroupDescription)
	assert.Equal(t, "codey-abc12-"+shortSHA("codey-mini"), items[0].InstanceID)

	assert.Equal(t, "cloudscale-ssd", items[1].ProductID)
	assert.Equal(t, "10", items[1].Value)
	assert.Equal(t, "si-84c17714 on Cloudscale RMA", items[1].ItemDescription)
	assert.Equal(t, "Servala Service: Codey", items[1].ItemGroupDescription)
	assert.Equal(t, "codey-abc12-"+shortSHA("cloudscale-ssd"), items[1].InstanceID)
}

// TestBillingService_Servala_OptsItemsIgnored verifies that caller-supplied opts.Items are
// not included for Servala deployments — the annotation is the sole source of truth.
// This prevents duplication when an addon (e.g. Collabora) is already declared in the annotation.
func TestBillingService_Servala_OptsItemsIgnored(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_annotation.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "codey-abc12",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "test-ns",
				"crossplane.io/claim-name":      "my-codey",
			},
			Annotations: map[string]string{
				"billing.servala.com/salesOrderID": "S12509",
				// annotation already contains the base product and the Collabora addon
				"billing.servala.com/items": `{"items":[{"productID":"nc-servala","value":"1"},{"productID":"nc-office-servala","value":"1"}]}`,
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	// Simulate Nextcloud billing.go passing the Collabora addon item via opts
	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
		Items: []v1.ItemSpec{
			{ProductID: "appcat-vshn-nextcloud-office-besteffort", Value: "1"},
		},
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_NORMAL, result.Severity)

	bs := &v1.BillingService{}
	require.NoError(t, svc.GetDesiredKubeObject(bs, "codey-abc12-billing-service"))

	// Must contain exactly the 2 items from the annotation — not 3 (no duplicate from opts.Items)
	require.Len(t, bs.Spec.Odoo.Items, 2)
	assert.Equal(t, "nc-servala", bs.Spec.Odoo.Items[0].ProductID)
	assert.Equal(t, "codey-abc12-"+shortSHA("nc-servala"), bs.Spec.Odoo.Items[0].InstanceID)
	assert.Equal(t, "nc-office-servala", bs.Spec.Odoo.Items[1].ProductID)
	assert.Equal(t, "codey-abc12-"+shortSHA("nc-office-servala"), bs.Spec.Odoo.Items[1].InstanceID)
}

func TestBillingService_JSONAnnotation_MissingItemsAnnotation(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_no_annotations.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "codey-abc12",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "test-ns",
				"crossplane.io/claim-name":      "my-codey",
			},
			// no billing.servala.com/items annotation
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_WARNING, result.Severity)
}

func TestBillingService_JSONAnnotation_InvalidJSON(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_no_annotations.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "codey-abc12",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "test-ns",
				"crossplane.io/claim-name":      "my-codey",
			},
			Annotations: map[string]string{
				"billing.servala.com/items": `not valid json`,
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_WARNING, result.Severity)
}

func TestBillingService_JSONAnnotation_EmptyItemsArray(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_no_annotations.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "codey-abc12",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "test-ns",
				"crossplane.io/claim-name":      "my-codey",
			},
			Annotations: map[string]string{
				"billing.servala.com/items": `{"items":[]}`,
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_WARNING, result.Severity)
}

func TestBillingService_JSONAnnotation_EmptyProductID(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_no_annotations.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "codey-abc12",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "test-ns",
				"crossplane.io/claim-name":      "my-codey",
			},
			Annotations: map[string]string{
				"billing.servala.com/items": `{"items":[{"productID":"","value":"1"}]}`,
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_WARNING, result.Severity)
}

func TestBillingService_NoPrefix_ComputedProductID(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_no_prefix.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nc-xyz",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "prod-ns",
				"crossplane.io/claim-name":      "my-nc",
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_NORMAL, result.Severity)

	bs := &v1.BillingService{}
	err := svc.GetDesiredKubeObject(bs, "nc-xyz-billing-service")
	require.NoError(t, err)

	require.Len(t, bs.Spec.Odoo.Items, 1)
	item := bs.Spec.Odoo.Items[0]
	assert.Equal(t, "appcat-vshn-nextcloud-besteffort", item.ProductID)
	assert.Equal(t, "1", item.Value)
	assert.Equal(t, "APPUiO Managed - Cluster: c-managed-test / Namespace: prod-ns", item.ItemDescription)
	assert.Equal(t, "my-nc", item.ItemGroupDescription)
	assert.Equal(t, "nc-xyz-"+shortSHA("appcat-vshn-nextcloud-besteffort"), item.InstanceID)
}

func TestBillingService_NoPrefix_GuaranteedSLA(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_no_prefix.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nc-xyz",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "prod-ns",
				"crossplane.io/claim-name":      "my-nc",
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 3,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.Guaranteed},
			},
		},
	}

	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_NORMAL, result.Severity)

	bs := &v1.BillingService{}
	require.NoError(t, svc.GetDesiredKubeObject(bs, "nc-xyz-billing-service"))

	require.Len(t, bs.Spec.Odoo.Items, 1)
	item := bs.Spec.Odoo.Items[0]
	assert.Equal(t, "appcat-vshn-nextcloud-guaranteed", item.ProductID)
	assert.Equal(t, "3", item.Value)
	assert.Equal(t, "nc-xyz-"+shortSHA("appcat-vshn-nextcloud-guaranteed"), item.InstanceID)
}

func TestBillingService_NoPrefix_WithAddonItem(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/billing_service_no_prefix.yaml")

	comp := &v1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nc-xyz",
			Labels: map[string]string{
				"crossplane.io/claim-namespace": "prod-ns",
				"crossplane.io/claim-name":      "my-nc",
			},
		},
		Spec: v1.VSHNNextcloudSpec{
			Parameters: v1.VSHNNextcloudParameters{
				Instances: 1,
				Service:   v1.VSHNNextcloudServiceSpec{ServiceLevel: v1.BestEffort},
			},
		},
	}

	// Simulate Nextcloud billing.go passing the Collabora addon item via opts
	result := CreateOrUpdateBillingServiceWithOptions(context.Background(), svc, comp, BillingServiceOptions{
		ResourceNameSuffix: "-billing-service",
		Items: []v1.ItemSpec{
			{ProductID: "appcat-vshn-nextcloud-office-besteffort", Value: "1"},
		},
	})
	assert.Equal(t, xfnproto.Severity_SEVERITY_NORMAL, result.Severity)

	bs := &v1.BillingService{}
	require.NoError(t, svc.GetDesiredKubeObject(bs, "nc-xyz-billing-service"))

	// Must contain both the addon item (from opts) and the computed base product item
	require.Len(t, bs.Spec.Odoo.Items, 2)

	addon := bs.Spec.Odoo.Items[0]
	assert.Equal(t, "appcat-vshn-nextcloud-office-besteffort", addon.ProductID)
	assert.Equal(t, "1", addon.Value)
	assert.Equal(t, "APPUiO Managed - Cluster: c-managed-test / Namespace: prod-ns", addon.ItemDescription)
	assert.Equal(t, "my-nc", addon.ItemGroupDescription)
	assert.Equal(t, "nc-xyz-"+shortSHA("appcat-vshn-nextcloud-office-besteffort"), addon.InstanceID)

	base := bs.Spec.Odoo.Items[1]
	assert.Equal(t, "appcat-vshn-nextcloud-besteffort", base.ProductID)
	assert.Equal(t, "nc-xyz-"+shortSHA("appcat-vshn-nextcloud-besteffort"), base.InstanceID)
}
