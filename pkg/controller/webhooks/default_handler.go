package webhooks

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	appcatruntime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type DefaultWebhookHandler struct {
	client    client.Client
	log       logr.Logger
	withQuota bool
	obj       runtime.Object
	name      string
	gk        schema.GroupKind
	gr        schema.GroupResource
}

var _ webhook.CustomValidator = &DefaultWebhookHandler{}

// SetupWebhookHandlerWithManager registers the validation webhook with the manager.
func New(mgrClient client.Client, logger logr.Logger, withQuota bool, obj runtime.Object, name string, gk schema.GroupKind, gr schema.GroupResource) *DefaultWebhookHandler {

	return &DefaultWebhookHandler{
		client:    mgrClient,
		log:       logger,
		withQuota: withQuota,
		name:      name,
		obj:       obj,
		gk:        gk,
		gr:        gr,
	}
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *DefaultWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	comp, ok := obj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	providerConfigErrs := r.ValidateProviderConfig(ctx, comp)
	if len(providerConfigErrs) > 0 {
		allErrs = append(allErrs, providerConfigErrs...)
	}

	if r.withQuota {
		quotaErrs := r.checkQuotas(ctx, comp, true)
		if quotaErrs != nil {
			allErrs = append(allErrs, &field.Error{
				Field: "quota",
				Detail: fmt.Sprintf("quota check failed: %s",
					quotaErrs.Error()),
				BadValue: "*your namespace quota*",
				Type:     field.ErrorTypeForbidden,
			})
		}
	}

	// We aggregate and return all errors at the same time.
	// So the user is aware of all broken parameters.
	// But at the same time, if any of these fail we cannot do proper quota checks anymore.
	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			r.gk,
			comp.GetName(),
			allErrs,
		)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *DefaultWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	comp, ok := newObj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	if comp.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	providerConfigErrs := r.ValidateProviderConfig(ctx, comp)
	if len(providerConfigErrs) > 0 {
		allErrs = append(allErrs, providerConfigErrs...)
	}

	if r.withQuota {
		quotaErrs := r.checkQuotas(ctx, comp, true)
		if quotaErrs != nil {
			allErrs = append(allErrs, &field.Error{
				Field: "quota",
				Detail: fmt.Sprintf("quota check failed: %s",
					quotaErrs.Error()),
				BadValue: "*your namespace quota*",
				Type:     field.ErrorTypeForbidden,
			})
		}
	}

	// We aggregate and return all errors at the same time.
	// So the user is aware of all broken parameters.
	// But at the same time, if any of these fail we cannot do proper quota checks anymore.
	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			r.gk,
			comp.GetName(),
			allErrs,
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *DefaultWebhookHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}

	comp, ok := obj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	allErrs = GetClaimDeletionProtection(comp.GetSecurity(), allErrs)

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			r.gk,
			comp.GetName(),
			allErrs,
		)
	}
	return nil, nil
}

// checkQuotas will read the plan if it's set and then check if any other size parameters are overwriten
func (r *DefaultWebhookHandler) checkQuotas(ctx context.Context, comp common.Composite, checkNamespaceQuota bool) *apierrors.StatusError {

	var fieldErr *field.Error
	instances := int64(comp.GetInstances())
	allErrs := field.ErrorList{}
	resources := utils.Resources{}

	if comp.GetSize().Plan != "" {
		var err error
		resources, err = utils.FetchPlansFromCluster(ctx, r.client, "vshn"+r.name+"plans", comp.GetSize().Plan)
		if err != nil {
			return apierrors.NewInternalError(err)
		}
	}

	isLegacy := false
	if r.name == "redis" {
		isLegacy = true
	}
	r.addPathsToResources(&resources, isLegacy)

	if comp.GetSize().CPU != "" {
		resources.CPULimits, fieldErr = parseResource(resources.CPULimitsPath, comp.GetSize().CPU, "not a valid cpu size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	if comp.GetSize().Requests.CPU != "" {
		resources.CPURequests, fieldErr = parseResource(resources.CPURequestsPath, comp.GetSize().Requests.CPU, "not a valid cpu size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	if comp.GetSize().Memory != "" {
		resources.MemoryLimits, fieldErr = parseResource(resources.MemoryLimitsPath, comp.GetSize().Memory, "not a valid memory size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	if comp.GetSize().Requests.Memory != "" {
		resources.MemoryRequests, fieldErr = parseResource(resources.MemoryRequestsPath, comp.GetSize().Requests.Memory, "not a valid memory size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	if comp.GetSize().Disk != "" {
		resources.Disk, fieldErr = parseResource(resources.DiskPath, comp.GetSize().Disk, "not a valid cpu size")
		if fieldErr != nil {
			allErrs = append(allErrs, fieldErr)
		}
	}

	// We aggregate and return all errors at the same time.
	// So the user is aware of all broken parameters.
	// But at the same time, if any of these fail we cannot do proper quota checks anymore.
	if len(allErrs) != 0 {
		return apierrors.NewInvalid(
			r.gk,
			comp.GetName(),
			allErrs,
		)
	}

	resources.MultiplyBy(instances)

	checker := quotas.NewQuotaChecker(
		r.client,
		comp.GetName(),
		comp.GetNamespace(),
		comp.GetInstanceNamespace(),
		resources,
		r.gr,
		r.gk,
		checkNamespaceQuota,
		instances,
	)

	return checker.CheckQuotas(ctx)
}

func (r *DefaultWebhookHandler) addPathsToResources(res *utils.Resources, isLegacy bool) {
	basePath := field.NewPath("spec", "parameters", "size")

	if isLegacy {
		res.CPULimitsPath = basePath.Child("CPULimits")
		res.CPURequestsPath = basePath.Child("CPURequests")
		res.MemoryLimitsPath = basePath.Child("memoryLimits")
		res.MemoryRequestsPath = basePath.Child("memoryLimits")
	} else {
		res.CPULimitsPath = basePath.Child("CPU")
		res.CPURequestsPath = basePath.Child("Requests", "CPU")
		res.MemoryLimitsPath = basePath.Child("Memory")
		res.MemoryRequestsPath = basePath.Child("Requests", "Memory")
	}
	res.DiskPath = basePath.Child("disk")

}

// k8s limitation is 52 characters, our longest postfix we add is 15 character, therefore 37 chracters is the maximum length
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
func (r *DefaultWebhookHandler) validateResourceNameLength(name string, lenght int) error {
	if len(name) > lenght {
		return fmt.Errorf("%d/%d chars.\n\tWe add various postfixes and CronJob name length has it's own limitations: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names", len(name), lenght)
	}
	return nil
}

// ValidateProviderConfig validates that the appcat.vshn.io/provider-config label, if set,
// refers to valid kubernetes and helm provider configs with existing secrets
func (r *DefaultWebhookHandler) ValidateProviderConfig(ctx context.Context, comp common.Composite) field.ErrorList {
	allErrs := field.ErrorList{}

	labels := comp.GetLabels()
	if labels == nil {
		return allErrs
	}

	providerConfigName, exists := labels[appcatruntime.ProviderConfigLabel]
	if !exists || providerConfigName == "" {
		// No provider config specified, validation passes
		return allErrs
	}

	labelPath := field.NewPath("metadata", "labels", appcatruntime.ProviderConfigLabel)

	// Validate kubernetes provider config
	kubeProviderConfig := &unstructured.Unstructured{}
	kubeProviderConfig.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubernetes.crossplane.io",
		Version: "v1alpha1",
		Kind:    "ProviderConfig",
	})

	err := r.client.Get(ctx, client.ObjectKey{
		Name: providerConfigName,
	}, kubeProviderConfig)

	if err != nil {
		if apierrors.IsNotFound(err) {
			allErrs = append(allErrs, &field.Error{
				Field:    labelPath.String(),
				Detail:   fmt.Sprintf("kubernetes ProviderConfig %q not found", providerConfigName),
				BadValue: providerConfigName,
				Type:     field.ErrorTypeNotFound,
			})
		} else {
			allErrs = append(allErrs, &field.Error{
				Field:    labelPath.String(),
				Detail:   fmt.Sprintf("failed to get kubernetes ProviderConfig %q: %v", providerConfigName, err),
				BadValue: providerConfigName,
				Type:     field.ErrorTypeInternal,
			})
		}
	} else {
		// Validate the secret referenced by the kubernetes provider config
		if secretErr := r.validateProviderConfigSecret(ctx, kubeProviderConfig, "kubernetes", providerConfigName, labelPath); secretErr != nil {
			allErrs = append(allErrs, secretErr)
		}
	}

	// Validate helm provider config
	helmProviderConfig := &unstructured.Unstructured{}
	helmProviderConfig.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "helm.crossplane.io",
		Version: "v1beta1",
		Kind:    "ProviderConfig",
	})

	err = r.client.Get(ctx, client.ObjectKey{
		Name: providerConfigName,
	}, helmProviderConfig)

	if err != nil {
		if apierrors.IsNotFound(err) {
			allErrs = append(allErrs, &field.Error{
				Field:    labelPath.String(),
				Detail:   fmt.Sprintf("helm ProviderConfig %q not found", providerConfigName),
				BadValue: providerConfigName,
				Type:     field.ErrorTypeNotFound,
			})
		} else {
			allErrs = append(allErrs, &field.Error{
				Field:    labelPath.String(),
				Detail:   fmt.Sprintf("failed to get helm ProviderConfig %q: %v", providerConfigName, err),
				BadValue: providerConfigName,
				Type:     field.ErrorTypeInternal,
			})
		}
	} else {
		// Validate the secret referenced by the helm provider config
		if secretErr := r.validateProviderConfigSecret(ctx, helmProviderConfig, "helm", providerConfigName, labelPath); secretErr != nil {
			allErrs = append(allErrs, secretErr)
		}
	}

	return allErrs
}

// validateProviderConfigSecret validates that the secret referenced by a provider config exists
func (r *DefaultWebhookHandler) validateProviderConfigSecret(ctx context.Context, providerConfig *unstructured.Unstructured, providerType, providerConfigName string, labelPath *field.Path) *field.Error {
	// Extract the secret reference from the provider config
	credentials, found, err := unstructured.NestedMap(providerConfig.UnstructuredContent(), "spec", "credentials")
	if err != nil {
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("failed to parse %s ProviderConfig %q credentials: %v", providerType, providerConfigName, err),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInternal,
		}
	}

	if !found {
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("%s ProviderConfig %q has no credentials configured", providerType, providerConfigName),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInvalid,
		}
	}

	secretRef, found, err := unstructured.NestedMap(credentials, "secretRef")
	if err != nil {
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("failed to parse %s ProviderConfig %q secretRef: %v", providerType, providerConfigName, err),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInternal,
		}
	}

	if !found {
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("%s ProviderConfig %q has no secretRef configured", providerType, providerConfigName),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInvalid,
		}
	}

	secretName, found, err := unstructured.NestedString(secretRef, "name")
	if err != nil || !found || secretName == "" {
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("%s ProviderConfig %q secretRef has no name", providerType, providerConfigName),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInvalid,
		}
	}

	secretNamespace, found, err := unstructured.NestedString(secretRef, "namespace")
	if err != nil || !found || secretNamespace == "" {
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("%s ProviderConfig %q secretRef has no namespace", providerType, providerConfigName),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInvalid,
		}
	}

	// Check if the secret exists
	secret := &corev1.Secret{}
	err = r.client.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: secretNamespace,
	}, secret)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return &field.Error{
				Field:    labelPath.String(),
				Detail:   fmt.Sprintf("%s ProviderConfig %q references secret %s/%s which does not exist", providerType, providerConfigName, secretNamespace, secretName),
				BadValue: providerConfigName,
				Type:     field.ErrorTypeNotFound,
			}
		}
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("failed to get secret %s/%s referenced by %s ProviderConfig %q: %v", secretNamespace, secretName, providerType, providerConfigName, err),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInternal,
		}
	}

	return nil
}
