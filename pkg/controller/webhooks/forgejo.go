package webhooks

import (
	"context"
	"fmt"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnforgejo,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnforgejos,versions=v1,name=vshnforgejo.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnforgejos,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnforgejos/status,verbs=get;list;watch;patch;update

var (
	forgejoGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNForgejo"}
	forgejoGR = schema.GroupResource{Group: forgejoGK.Group, Resource: "vshnforgejo"}
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
			),
		}).
		Complete()
}

func (n *ForgejoWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	warning, err := n.DefaultWebhookHandler.ValidateCreate(ctx, obj)
	if warning != nil || err != nil {
		return warning, err
	}

	forgejo, ok := obj.(*vshnv1.VSHNForgejo)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNForgejo object")
	}

	if forgejo.Spec.Parameters.Service.FQDN == nil {
		return nil, fmt.Errorf("FQDN must not be empty")
	}

	if len(forgejo.Spec.Parameters.Service.FQDN) == 0 {
		return nil, fmt.Errorf("FQDN must not be empty")
	}

	if err := validateFQDNs(forgejo.Spec.Parameters.Service.FQDN); err != nil {
		return nil, err
	}

	return nil, nil
}

func (n *ForgejoWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	warning, err := n.DefaultWebhookHandler.ValidateUpdate(ctx, oldObj, newObj)
	if warning != nil || err != nil {
		return warning, err
	}

	forgejo, ok := newObj.(*vshnv1.VSHNForgejo)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNForgejo object")
	}

	if forgejo.Spec.Parameters.Service.FQDN == nil {
		return nil, fmt.Errorf("FQDN must not be empty")
	}

	if len(forgejo.Spec.Parameters.Service.FQDN) == 0 {
		return nil, fmt.Errorf("FQDN must not be empty")
	}

	if err := validateFQDNs(forgejo.Spec.Parameters.Service.FQDN); err != nil {
		return nil, err
	}

	return nil, nil
}
