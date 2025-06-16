package webhooks

import (
	"context"
	"fmt"
	"strings"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnkeycloak,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnkeycloaks,versions=v1,name=vshnkeycloak.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnkeycloaks,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnkeycloaks/status,verbs=get;list;watch;patch;update

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
			),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (n *KeycloakWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	warning, err := n.DefaultWebhookHandler.ValidateCreate(ctx, obj)
	if warning != nil || err != nil {
		return warning, err
	}

	keycloak, ok := obj.(*vshnv1.VSHNKeycloak)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNKeycloak object")
	}
	if err := validateCustomFilePaths(keycloak.Spec.Parameters.Service.CustomFiles); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *KeycloakWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	_, ok := oldObj.(*vshnv1.VSHNKeycloak)
	if !ok {
		return nil, fmt.Errorf("not a valid VSHNKeycloak object")
	}
	newKeycloak, ok := newObj.(*vshnv1.VSHNKeycloak)
	if !ok {
		return nil, fmt.Errorf("not a valid VSHNKeycloak object")
	}

	if err := validateCustomFilePaths(newKeycloak.Spec.Parameters.Service.CustomFiles); err != nil {
		return nil, err
	}

	return p.DefaultWebhookHandler.ValidateUpdate(ctx, oldObj, newObj)
}

func validateCustomFilePaths(customFiles []vshnv1.VSHNKeycloakCustomFile) error {
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
