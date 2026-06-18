package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apixv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnkeycloak,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnkeycloaks,versions=v1,name=vshnkeycloak.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnkeycloaks,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnkeycloaks/status,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=apiextensions.crossplane.io,resources=compositionrevisions,verbs=get;list;watch

var (
	keycloakGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNKeycloak"}
	keycloakGR = schema.GroupResource{Group: keycloakGK.Group, Resource: "vshnkeycloak"}
)

var _ webhook.CustomValidator = &KeycloakWebhookHandler{}

// Folders that may not be replaced by the custom files init container
// https://www.keycloak.org/server/directory-structure#_directory_structure
var keycloakRootFolders = []string{
	"providers",
	"themes",
	"lib",
	"conf",
	"bin",
}

type KeycloakWebhookHandler struct {
	DefaultWebhookHandler
}

var _ webhook.CustomValidator = &KeycloakWebhookHandler{}

// SetupKeycloakWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupKeycloakWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNKeycloak{}).
		WithValidator(&KeycloakWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("keycloak"),
				withQuota,
				&vshnv1.VSHNKeycloak{},
				"keycloak",
				keycloakGK,
				keycloakGR,
				maxNestedNameLength,
			),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (n *KeycloakWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	keycloak, ok := obj.(*vshnv1.VSHNKeycloak)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNKeycloak object")
	}

	allErrs := newFielErrors(keycloak.GetName(), keycloakGK)

	defaultWarnings, err := n.DefaultWebhookHandler.ValidateCreate(ctx, obj)
	if err != nil {
		tmpErr := err.(*fieldErrors)
		allErrs.Add(tmpErr.List()...)
	}

	if err := validateCustomImageMutualExclusion(keycloak); err != nil {
		allErrs.Add(err)
	}

	if err := validateCustomFileObject(keycloak); err != nil {
		allErrs.Add(err)
	}

	warnings := append(isDeprecatedFieldInUse(keycloak), warnPinImageTagIgnoredForCustomImage(keycloak)...)
	warnings = append(warnings, n.warnRelativePathDisablesOptimized(ctx, keycloak)...)
	return append(defaultWarnings, warnings...), allErrs.Get()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *KeycloakWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldKeycloak, ok := oldObj.(*vshnv1.VSHNKeycloak)
	if !ok {
		return nil, fmt.Errorf("not a valid VSHNKeycloak object")
	}
	newKeycloak, ok := newObj.(*vshnv1.VSHNKeycloak)
	if !ok {
		return nil, fmt.Errorf("not a valid VSHNKeycloak object")
	}

	if newKeycloak.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	allErrs := newFielErrors(newKeycloak.GetName(), keycloakGK)

	defaultWarnings, err := p.DefaultWebhookHandler.ValidateUpdate(ctx, oldObj, newObj)
	if err != nil {
		tmpErr := err.(*fieldErrors)
		allErrs.Add(tmpErr.List()...)
	}

	if err := validateCustomImageMutualExclusion(newKeycloak); err != nil {
		allErrs.Add(err)
	}

	if err := validateCustomFileObject(newKeycloak); err != nil {
		allErrs.Add(err)
	}

	if newKeycloak.Spec.Parameters.Service.PostgreSQLParameters != nil && oldKeycloak.Spec.Parameters.Service.PostgreSQLParameters != nil {
		newEncryption := &newKeycloak.Spec.Parameters.Service.PostgreSQLParameters.Encryption
		oldEncryption := &oldKeycloak.Spec.Parameters.Service.PostgreSQLParameters.Encryption
		fieldPath := "spec.parameters.service.postgreSQLParameters.encryption.enabled"
		if err := validatePostgreSQLEncryptionChanges(newEncryption, oldEncryption, fieldPath); err != nil {
			allErrs.Add(err)
		}
	}

	warnings := append(isDeprecatedFieldInUse(newKeycloak), warnPinImageTagIgnoredForCustomImage(newKeycloak)...)
	warnings = append(warnings, p.warnRelativePathDisablesOptimized(ctx, newKeycloak)...)
	return append(defaultWarnings, warnings...), allErrs.Get()
}

func validateCustomImageMutualExclusion(keycloak *vshnv1.VSHNKeycloak) *field.Error {
	if keycloak.Spec.Parameters.Service.CustomImage.Image != "" &&
		keycloak.Spec.Parameters.Service.CustomizationImage.Image != "" {
		return field.Invalid(
			field.NewPath("spec", "parameters", "service", "customImage", "image"),
			keycloak.Spec.Parameters.Service.CustomImage.Image,
			"cannot set both 'customImage' and 'customizationImage': 'customizationImage' is deprecated, use 'customImage' instead",
		)
	}
	return nil
}

func validateCustomFileObject(keycloak *vshnv1.VSHNKeycloak) *field.Error {
	if len(keycloak.Spec.Parameters.Service.CustomFiles) > 0 {
		if keycloak.Spec.Parameters.Service.CustomizationImage.Image == "" {
			return field.Invalid(field.NewPath("spec", "parameters", "service", "customizationImage", "image"), "", "custom files have been defined, but no customization image")
		}
	}

	return validateCustomFilePaths(keycloak.Spec.Parameters.Service.CustomFiles)
}

func validateCustomFilePaths(customFiles []vshnv1.VSHNKeycloakCustomFile) *field.Error {
	fieldPath := field.NewPath("spec", "parameters", "service", "customFiles")
	for i, customFile := range customFiles {
		if customFile.Source == "" {
			return field.Invalid(
				fieldPath.Index(i).Child("source"),
				customFile.Source,
				"No source specified",
			)
		}

		if customFile.Destination == "" {
			return field.Invalid(
				fieldPath.Index(i).Child("destination"),
				customFile.Destination,
				"No destination specified",
			)
		}

		if strings.Contains(customFile.Destination, "..") {
			return field.Invalid(
				fieldPath.Index(i).Child("destination"),
				customFile.Destination,
				"May not perform path traversal",
			)
		}

		for _, folder := range keycloakRootFolders {
			if strings.HasPrefix(strings.TrimPrefix(customFile.Destination, "/"), folder) {
				return field.Invalid(
					fieldPath.Index(i).Child("destination"),
					customFile.Destination,
					"Destination cannot be a keycloak root folder",
				)
			}
		}
	}

	return nil
}

func warnPinImageTagIgnoredForCustomImage(keycloak *vshnv1.VSHNKeycloak) admission.Warnings {
	if keycloak.Spec.Parameters.Service.CustomImage.Image != "" &&
		keycloak.Spec.Parameters.Maintenance.PinImageTag != "" {
		return admission.Warnings{fmt.Sprintf(
			"%s has no effect when %s is set; the image tag from customImage takes precedence",
			field.NewPath("spec", "parameters", "maintenance", "pinImageTag").String(),
			field.NewPath("spec", "parameters", "service", "customImage", "image").String(),
		)}
	}
	return nil
}

const (
	keycloakImagesOptimizedInput = "keycloak_images_optimized"
	keycloakServiceIDLabel       = "metadata.appcat.vshn.io/serviceID"
	keycloakServiceID            = "vshn-keycloak"
)

// warnRelativePathDisablesOptimized warns when a non-root relativePath is combined with optimized
// Keycloak images. http-relative-path is a Keycloak build-time option, so for such instances the
// composition omits --optimized and Keycloak rebuilds at startup (slower). It checks the latest
// composition revision and only warns when keycloak_images_optimized is actually "true".
func (h *KeycloakWebhookHandler) warnRelativePathDisablesOptimized(ctx context.Context, keycloak *vshnv1.VSHNKeycloak) admission.Warnings {
	if strings.TrimSuffix(keycloak.Spec.Parameters.Service.RelativePath, "/") == "" {
		return nil
	}
	if !h.optimizedImagesEnabled(ctx) {
		return nil
	}
	return admission.Warnings{fmt.Sprintf(
		"%s is not '/': optimized Keycloak images are enabled, so --optimized is omitted for this instance "+
			"and Keycloak performs a slower runtime build at startup (http-relative-path is a build-time option)",
		field.NewPath("spec", "parameters", "service", "relativePath").String(),
	)}
}

// optimizedImagesEnabled reports whether the latest Keycloak composition revision sets the function
// input keycloak_images_optimized to "true". The latest revision is what Crossplane binds to new
// instances by default, so it reflects what an instance will actually run. Returns false on any
// error so the webhook never blocks admission.
func (h *KeycloakWebhookHandler) optimizedImagesEnabled(ctx context.Context) bool {
	list := &apixv1.CompositionRevisionList{}
	if err := h.client.List(ctx, list, client.MatchingLabels{keycloakServiceIDLabel: keycloakServiceID}); err != nil {
		return false
	}

	var latest *apixv1.CompositionRevision
	for i := range list.Items {
		r := &list.Items[i]
		if latest == nil || r.Spec.Revision > latest.Spec.Revision {
			latest = r
		}
	}
	if latest == nil {
		return false
	}

	for _, step := range latest.Spec.Pipeline {
		if step.Input == nil {
			continue
		}
		input := &corev1.ConfigMap{}
		if err := json.Unmarshal(step.Input.Raw, input); err != nil {
			continue
		}
		return input.Data[keycloakImagesOptimizedInput] == "true"
	}
	return false
}

func isDeprecatedFieldInUse(comp *vshnv1.VSHNKeycloak) admission.Warnings {
	var warnings admission.Warnings
	if comp.Spec.Parameters.Service.CustomEnvVariablesRef != nil {
		warnings = append(warnings, fmt.Sprintf(
			"Field 'customEnvVariablesRef' in %s has been deprecated, please use 'envFrom' instead.",
			field.NewPath("spec", "parameters", "service").String(),
		))
	}
	if comp.Spec.Parameters.Service.CustomizationImage.Image != "" {
		warnings = append(warnings, fmt.Sprintf(
			"Field 'customizationImage' in %s has been deprecated, please use 'customImage' instead.",
			field.NewPath("spec", "parameters", "service").String(),
		))
	}
	return warnings
}
