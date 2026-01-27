package webhooks

import (
	"context"
	"fmt"
	"slices"
	"strings"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnforgejo,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnforgejoes,versions=v1,name=vshnforgejo.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnforgejoes,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnforgejoes/status,verbs=get;list;watch;patch;update

var (
	forgejoGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNForgejo"}
	forgejoGR = schema.GroupResource{Group: forgejoGK.Group, Resource: "vshnforgejo"}

	denied_mailer_protocols = []string{
		"smtp+unix",
		"sendmail",
	}
)

var _ webhook.CustomValidator = &ForgejoWebhookHandler{}

type ForgejoWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupForgejoWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupForgejoWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNForgejo{}).
		WithValidator(&ForgejoWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("forgejo"),
				withQuota,
				&vshnv1.VSHNForgejo{},
				"forgejo",
				forgejoGK,
				forgejoGR,
				maxNestedNameLength,
			),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (n *ForgejoWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	forgejo, ok := obj.(*vshnv1.VSHNForgejo)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNForgejo object")
	}

	allErrs := newFielErrors(forgejo.GetName(), forgejoGK)

	warning, err := n.DefaultWebhookHandler.ValidateCreate(ctx, obj)
	if err != nil {
		tmpErr := err.(*fieldErrors)
		allErrs.Add(tmpErr.List()...)
	}
	if warning != nil && err == nil {
		return warning, nil
	}

	if err := validateFQDNs(forgejo.Spec.Parameters.Service.FQDN); err != nil {
		allErrs.Add(err)
	}

	if err := validateForgejoConfig(forgejo.Spec.Parameters.Service.ForgejoSettings); err != nil {
		allErrs.Add(err)
	}

	return nil, allErrs.Get()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *ForgejoWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	_, ok := oldObj.(*vshnv1.VSHNForgejo)
	if !ok {
		return nil, fmt.Errorf("not a valid VSHNForgejo object")
	}
	newForgejo, ok := newObj.(*vshnv1.VSHNForgejo)
	if !ok {
		return nil, fmt.Errorf("not a valid VSHNForgejo object")
	}

	if newForgejo.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	allErrs := newFielErrors(newForgejo.GetName(), forgejoGK)

	warnings, err := p.DefaultWebhookHandler.ValidateUpdate(ctx, oldObj, newObj)
	if err != nil {
		tmpErr := err.(*fieldErrors)
		allErrs.Add(tmpErr.List()...)
	}
	if warnings != nil && err == nil {
		return warnings, nil
	}

	if err := validateFQDNs(newForgejo.Spec.Parameters.Service.FQDN); err != nil {
		allErrs.Add(err)
	}

	if err := validateForgejoConfig(newForgejo.Spec.Parameters.Service.ForgejoSettings); err != nil {
		allErrs.Add(err)
	}

	return nil, allErrs.Get()
}

func validateForgejoConfig(settings vshnv1.VSHNForgejoSettings) *field.Error {
	// Mailer
	for k, v := range settings.Config.Mailer {
		if strings.EqualFold(k, "PROTOCOL") {
			if slices.Contains(denied_mailer_protocols, v) {
				return field.Invalid(
					field.NewPath("spec", "paramaters", "service", "config", "mailer"),
					v,
					fmt.Sprintf("bad mailer.PROTOCOL specified: %s. May not be any of: %v", v, denied_mailer_protocols))
			}
		}
	}

	return nil
}
