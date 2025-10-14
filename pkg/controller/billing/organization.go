package billing

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// APPUiOControlSecretNamespace is the namespace where the APPUiO control plane secret is located
	APPUiOControlSecretNamespace = "syn-appcat"
	// APPUiOControlSecretName is the name of the secret containing kubeconfig for APPUiO control plane
	APPUiOControlSecretName = "appuio-control-sa"
)

// getExternalClusterClient creates a Kubernetes client from a kubeconfig stored in a secret
func getExternalClusterClient(ctx context.Context, localClient client.Client) (client.Client, error) {
	// Fetch the secret containing the kubeconfig
	secret := &corev1.Secret{}
	err := localClient.Get(ctx, types.NamespacedName{
		Namespace: APPUiOControlSecretNamespace,
		Name:      APPUiOControlSecretName,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", APPUiOControlSecretNamespace, APPUiOControlSecretName, err)
	}

	// Extract kubeconfig from secret
	kubeconfigData, ok := secret.Data["kubeconfig"]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s does not contain 'kubeconfig' key", APPUiOControlSecretNamespace, APPUiOControlSecretName)
	}

	// Create REST config from kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config from kubeconfig: %w", err)
	}

	// Create client
	externalClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create external cluster client: %w", err)
	}

	return externalClient, nil
}

// getSalesOrderFromOrganization fetches the salesOrderName from an Organization CR
func getSalesOrderFromOrganization(ctx context.Context, externalClient client.Client, organizationName string) (string, error) {
	// Define the GVK for organizations.organization.appuio.io
	gvk := schema.GroupVersionKind{
		Group:   "organization.appuio.io",
		Version: "v1",
		Kind:    "Organization",
	}

	org := &unstructured.Unstructured{}
	org.SetGroupVersionKind(gvk)

	err := externalClient.Get(ctx, types.NamespacedName{
		Name: organizationName,
	}, org)
	if err != nil {
		return "", fmt.Errorf("failed to get Organization %s: %w", organizationName, err)
	}

	// Extract status.salesOrderName
	salesOrderName, found, err := unstructured.NestedString(org.Object, "status", "salesOrderName")
	if err != nil {
		return "", fmt.Errorf("error extracting salesOrderName from Organization %s: %w", organizationName, err)
	}
	if !found || salesOrderName == "" {
		return "", fmt.Errorf("salesOrderName not found or empty in Organization %s", organizationName)
	}

	return salesOrderName, nil
}

// fetchSalesOrderFromAPPUiOControl fetches the sales order from the APPUiO control plane
// It creates a client to the external cluster using a secret and queries the Organization CR
func (b *BillingHandler) fetchSalesOrderFromAPPUiOControl(ctx context.Context, organizationName string) (string, error) {
	externalClient, err := getExternalClusterClient(ctx, b.Client)
	if err != nil {
		return "", fmt.Errorf("failed to create external cluster client: %w", err)
	}

	salesOrder, err := getSalesOrderFromOrganization(ctx, externalClient, organizationName)
	if err != nil {
		return "", fmt.Errorf("failed to get sales order from Organization: %w", err)
	}

	return salesOrder, nil
}
