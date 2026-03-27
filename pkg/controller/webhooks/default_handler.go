package webhooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	appcatruntime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;patch;update;delete
//+kubebuilder:rbac:groups=cloudscale.crossplane.io;kubernetes.crossplane.io;helm.crossplane.io;minio.crossplane.io;postgresql.sql.crossplane.io,resources=providerconfigs,verbs=get;list;watch

const (
	// because postgresql has a max of 30 and the
	// pg builder adds `-pg` to the composite...
	maxNestedNameLength   = 27
	maxResourceNameLength = 37
)

type DefaultWebhookHandler struct {
	client     client.Client
	log        logr.Logger
	withQuota  bool
	obj        runtime.Object
	name       string
	gk         schema.GroupKind
	gr         schema.GroupResource
	nameLength int
}

var _ webhook.CustomValidator = &DefaultWebhookHandler{}

func newFielErrors(compName string, compGK schema.GroupKind) *fieldErrors {
	return &fieldErrors{
		compGK:   compGK,
		compName: compName,
		errors:   field.ErrorList{},
	}
}

// fieldErrors is a helper struct to collect
// fieldErrors. So all invalid fields
// can be reported at once.
type fieldErrors struct {
	compGK   schema.GroupKind
	compName string
	errors   field.ErrorList
}

// Add adds a new field.Error to the internal list
func (f *fieldErrors) Add(err ...*field.Error) {
	f.errors = append(f.errors, err...)
}

// Get should be called when fieldErrors is returned.
// This ensures that we can do nil checks from the caller.
func (f *fieldErrors) Get() error {
	if len(f.errors) > 0 {
		return f
	}
	return nil
}

// wrap will wrap all the errors in an apierror.
func (f *fieldErrors) wrap() error {
	if len(f.errors) > 0 {
		return apierrors.NewInvalid(f.compGK, f.compName, f.errors)
	}
	return nil
}

// Error implements the error interface
func (f fieldErrors) Error() string {
	if f.wrap() != nil {
		return f.wrap().Error()
	}
	return ""
}

// List provides a function to get the underyling
// ErrorList. This is useful for nesting fieldErrors
// e.g. when calling a default handler in a service specific handler.
func (f *fieldErrors) List() field.ErrorList {
	return f.errors
}

// SetupWebhookHandlerWithManager registers the validation webhook with the manager.
func New(mgrClient client.Client, logger logr.Logger, withQuota bool, obj runtime.Object, name string, gk schema.GroupKind, gr schema.GroupResource, nameLength int) *DefaultWebhookHandler {
	return &DefaultWebhookHandler{
		client:     mgrClient,
		log:        logger,
		withQuota:  withQuota,
		name:       name,
		obj:        obj,
		gk:         gk,
		gr:         gr,
		nameLength: nameLength,
	}
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *DefaultWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	comp, ok := obj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	allErrs := newFielErrors(comp.GetName(), comp.GetObjectKind().GroupVersionKind().GroupKind())

	providerConfigErrs := r.ValidateProviderConfig(ctx, comp)
	if len(providerConfigErrs) > 0 {
		allErrs.Add(providerConfigErrs...)
	}

	if r.withQuota {
		quotaErrs := r.checkQuotas(ctx, comp, true)
		if quotaErrs != nil {
			allErrs.Add(&field.Error{
				Field: "quota",
				Detail: fmt.Sprintf("quota check failed: %s",
					quotaErrs.Error()),
				BadValue: "*your namespace quota*",
				Type:     field.ErrorTypeForbidden,
			})
		}
	}

	if err := r.validateResourceNameLength(comp.GetName()); err != nil {
		allErrs.Add(err)
	}

	warn := checkManualVersionManagementWarnings(comp.GetFullMaintenanceSchedule())
	return warn, allErrs.Get()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *DefaultWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	comp, ok := newObj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	oldComp, ok := oldObj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided old manifest is not a valid " + r.gk.Kind + " object")
	}

	if comp.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	allErrs := newFielErrors(comp.GetName(), comp.GetObjectKind().GroupVersionKind().GroupKind())

	// Check for disk downsizing
	if diskErr := r.ValidateDiskDownsizing(ctx, oldComp, comp, r.gk.Kind); diskErr != nil {
		allErrs.Add(diskErr)
	}

	providerConfigErrs := r.ValidateProviderConfig(ctx, comp)
	if len(providerConfigErrs) > 0 {
		allErrs.Add(providerConfigErrs...)
	}

	if r.withQuota {
		quotaErrs := r.checkQuotas(ctx, comp, true)
		if quotaErrs != nil {
			allErrs.Add(&field.Error{
				Field: "quota",
				Detail: fmt.Sprintf("quota check failed: %s",
					quotaErrs.Error()),
				BadValue: "*your namespace quota*",
				Type:     field.ErrorTypeForbidden,
			})
		}
	}

	warn := checkManualVersionManagementWarnings(comp.GetFullMaintenanceSchedule())
	return warn, allErrs.Get()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *DefaultWebhookHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	comp, ok := obj.(common.Composite)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid " + r.gk.Kind + " object")
	}

	allErrs := newFielErrors(comp.GetName(), obj.GetObjectKind().GroupVersionKind().GroupKind())

	if err := GetClaimDeletionProtection(comp.GetSecurity()); err != nil {
		allErrs.Add(err)
	}

	return nil, allErrs.Get()
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
func (r *DefaultWebhookHandler) validateResourceNameLength(name string) *field.Error {
	if len(name) > r.nameLength {
		return &field.Error{
			Field:    ".metadata.name",
			Detail:   fmt.Sprintf("Please shorten %s, currently it is: %d/%d chars", name, len(name), r.nameLength),
			BadValue: name,
			Type:     field.ErrorTypeTooLong,
		}
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

	_, foundSource, err := unstructured.NestedString(credentials, "source")
	if err != nil {
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("failed to parse %s ProviderConfig %q source: %v", providerType, providerConfigName, err),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInternal,
		}
	}

	secretRef, foundSecret, err := unstructured.NestedMap(credentials, "secretRef")
	if err != nil {
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("failed to parse %s ProviderConfig %q secretRef: %v", providerType, providerConfigName, err),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInternal,
		}
	}

	if foundSource == false && foundSecret == false {
		return &field.Error{
			Field:    labelPath.String(),
			Detail:   fmt.Sprintf("%s ProviderConfig %q has no secretRef or source configured", providerType, providerConfigName),
			BadValue: providerConfigName,
			Type:     field.ErrorTypeInvalid,
		}
	}
	if foundSource == true {
		return nil
	}

	if !foundSecret {
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

// ValidateDiskDownsizing checks if the disk size is being reduced and returns an error if so.
// This is a global validation function that can be called by any AppCat service webhook.
// It handles both explicit disk sizes and plan-based sizing.
func (r *DefaultWebhookHandler) ValidateDiskDownsizing(ctx context.Context, oldComp, newComp common.Composite, serviceName string) *field.Error {
	// Get the actual disk size from old configuration (plan or explicit)
	oldDiskSize, err := r.getEffectiveDiskSize(ctx, oldComp, serviceName)
	if err != nil {
		// If we can't determine the old size, skip validation to avoid blocking legitimate changes
		r.log.V(1).Info("Could not determine old disk size, skipping downsizing validation", "error", err)
		return nil
	}
	if oldDiskSize == "" {
		// No disk size configured, skip validation
		return nil
	}

	// Get the actual disk size from new configuration (plan or explicit)
	newDiskSize, err := r.getEffectiveDiskSize(ctx, newComp, serviceName)
	if err != nil {
		// Return error for invalid new configuration
		return &field.Error{
			Field:    "spec.parameters.size",
			Detail:   fmt.Sprintf("invalid size configuration: %s", err.Error()),
			BadValue: fmt.Sprintf("plan: %s, disk: %s", newComp.GetSize().Plan, newComp.GetSize().Disk),
			Type:     field.ErrorTypeInvalid,
		}
	}
	if newDiskSize == "" {
		// No disk size in new config, allow it
		return nil
	}

	// Parse both sizes as Kubernetes resource quantities
	oldQuantity, err := resource.ParseQuantity(oldDiskSize)
	if err != nil {
		// If we can't parse the old size, skip validation
		r.log.V(1).Info("Could not parse old disk size, skipping downsizing validation", "size", oldDiskSize, "error", err)
		return nil
	}

	newQuantity, err := resource.ParseQuantity(newDiskSize)
	if err != nil {
		// Return error for invalid new size format
		return &field.Error{
			Field:    "spec.parameters.size.disk",
			Detail:   fmt.Sprintf("invalid disk size format: %s", err.Error()),
			BadValue: newDiskSize,
			Type:     field.ErrorTypeInvalid,
		}
	}

	// Check if the new size is smaller than the old size
	if newQuantity.Cmp(oldQuantity) < 0 {
		return &field.Error{
			Field:    "spec.parameters.size",
			Detail:   fmt.Sprintf("disk downsizing not allowed for %s. Current: %s, requested: %s", serviceName, oldDiskSize, newDiskSize),
			BadValue: fmt.Sprintf("plan: %s, disk: %s", newComp.GetSize().Plan, newComp.GetSize().Disk),
			Type:     field.ErrorTypeForbidden,
		}
	}

	return nil
}

// getEffectiveDiskSize returns the actual disk size from either explicit disk parameter or plan.
// Returns empty string if no disk size is configured.
func (r *DefaultWebhookHandler) getEffectiveDiskSize(ctx context.Context, comp common.Composite, serviceName string) (string, error) {
	size := comp.GetSize()

	// If explicit disk size is set, use it
	if size.Disk != "" {
		return size.Disk, nil
	}

	// If plan is set, resolve the disk size from the plan
	if size.Plan != "" {
		planResources, err := utils.FetchPlansFromCluster(ctx, r.client, strings.ToLower(serviceName)+"plans", size.Plan)
		if err != nil {
			return "", fmt.Errorf("failed to fetch plan %s: %w", size.Plan, err)
		}

		if planResources.Disk.IsZero() {
			return "", nil
		}

		return planResources.Disk.String(), nil
	}

	// No disk size configured
	return "", nil
}

// checkManualVersionManagementWarnings returns warnings if manual version management flags are enabled
func checkManualVersionManagementWarnings(maintenance vshnv1.VSHNDBaaSMaintenanceScheduleSpec) admission.Warnings {
	var warnings admission.Warnings
	if maintenance.PinImageTag != "" {
		warnings = append(warnings,
			fmt.Sprintf("WARNING: Image tag pinned to %q at %s. You are responsible for version management. Downgrades are allowed at your own risk.",
				maintenance.PinImageTag,
				field.NewPath("spec", "parameters", "maintenance", "pinImageTag").String()))
	}
	if maintenance.DisableAppcatRelease {
		warnings = append(warnings,
			"WARNING: AppCat release updates disabled at "+
				field.NewPath("spec", "parameters", "maintenance", "disableAppcatRelease").String()+
				". This is strongly discouraged and may leave your instance without security patches.")
	}
	return warnings
}
