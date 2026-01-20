package webhooks

import (
	"context"
	"fmt"

	valid "github.com/asaskevich/govalidator"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnnextcloud,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnnextclouds,versions=v1,name=vshnnextcloud.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnnextclouds,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnnextclouds/status,verbs=get;list;watch;patch;update

var (
	nextcloudGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNNextcloud"}
	nextcloudGR = schema.GroupResource{Group: nextcloudGK.Group, Resource: "vshnnextcloud"}
)

var _ webhook.CustomValidator = &NextcloudWebhookHandler{}

type NextcloudWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupNextcloudWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupNextcloudWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNNextcloud{}).
		WithValidator(&NextcloudWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("nextcloud"),
				withQuota,
				&vshnv1.VSHNNextcloud{},
				"nextcloud",
				nextcloudGK,
				nextcloudGR,
				maxNestedNameLength,
			),
		}).
		Complete()
}

func (n *NextcloudWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	nx, ok := obj.(*vshnv1.VSHNNextcloud)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNPostgreSQL object")
	}

	allErrs := newFielErrors(nx.GetName(), nextcloudGK)

	warning, err := n.DefaultWebhookHandler.ValidateCreate(ctx, obj)
	if err != nil {
		tmpErr := err.(*fieldErrors)
		allErrs.Add(tmpErr.List()...)
	}
	if warning != nil && err == nil {
		return warning, nil
	}

	if err := validateFQDNs(nx.Spec.Parameters.Service.FQDN); err != nil {
		allErrs.Add(err)
	}

	if nx.Spec.Parameters.Service.Collabora.Enabled {
		if nx.Spec.Parameters.Service.Collabora.FQDN == "" {
			allErrs.Add(field.Invalid(
				field.NewPath("spec", "parameters", "service", "collabora", "fqdn"),
				"",
				"Collabora FQDN is required when Collabora is enabled",
			))
		}
		if err := validateFQDNs([]string{nx.Spec.Parameters.Service.Collabora.FQDN}); err != nil {
			allErrs.Add(err)
		}
	}

	return nil, allErrs.Get()
}

func (n *NextcloudWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	nx, ok := newObj.(*vshnv1.VSHNNextcloud)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNNextcloud object")
	}

	if nx.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	allErrs := newFielErrors(nx.GetName(), nextcloudGK)

	warning, err := n.DefaultWebhookHandler.ValidateUpdate(ctx, oldObj, newObj)
	if err != nil {
		tmpErr := err.(*fieldErrors)
		allErrs.Add(tmpErr.List()...)
	}
	if warning != nil && err == nil {
		return warning, err
	}

	oldNx, ok := oldObj.(*vshnv1.VSHNNextcloud)
	if !ok {
		return nil, fmt.Errorf("provided old manifest is not a valid VSHNNextcloud object")
	}

	if err := validateFQDNs(nx.Spec.Parameters.Service.FQDN); err != nil {
		allErrs.Add(err)
	}

	if nx.Spec.Parameters.Service.Collabora.Enabled {
		if nx.Spec.Parameters.Service.Collabora.FQDN == "" {
			allErrs.Add(field.Invalid(
				field.NewPath("spec", "parameters", "service", "collabora", "fqdn"),
				"",
				"Collabora FQDN is required when Collabora is enabled",
			))
		}
		if err := validateFQDNs([]string{nx.Spec.Parameters.Service.Collabora.FQDN}); err != nil {
			allErrs.Add(err)
		}
	}

	// Validate PostgreSQL encryption changes
	if nx.Spec.Parameters.Service.PostgreSQLParameters != nil && oldNx.Spec.Parameters.Service.PostgreSQLParameters != nil {
		newEncryption := &nx.Spec.Parameters.Service.PostgreSQLParameters.Encryption
		oldEncryption := &oldNx.Spec.Parameters.Service.PostgreSQLParameters.Encryption
		fieldPath := "spec.parameters.service.postgreSQLParameters.encryption.enabled"
		if err := validatePostgreSQLEncryptionChanges(newEncryption, oldEncryption, fieldPath); err != nil {
			allErrs.Add(err)
		}
	}

	return nil, allErrs.Get()
}

func validateFQDNs(fqdns []string) *field.Error {
	for _, fqdn := range fqdns {
		if !valid.IsDNSName(fqdn) {
			return field.Invalid(
				field.NewPath("spec", "parameters", "service", "fqdn"),
				fqdn,
				fmt.Sprintf("FQDN %s is not a valid DNS name", fqdn))
		}
	}
	return nil
}
