package webhooks

import (
	"context"
	"fmt"

	valid "github.com/asaskevich/govalidator"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			),
		}).
		Complete()
}

func (n *NextcloudWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	warning, err := n.DefaultWebhookHandler.ValidateCreate(ctx, obj)
	if warning != nil || err != nil {
		return warning, err
	}

	nx, ok := obj.(*vshnv1.VSHNNextcloud)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNPostgreSQL object")
	}

	if err := validateFQDNs(nx.Spec.Parameters.Service.FQDN); err != nil {
		return nil, err
	}

	if nx.Spec.Parameters.AddOns.Office.Enabled {
		if err := validateFQDNs([]string{nx.Spec.Parameters.AddOns.Office.FQDN}); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (n *NextcloudWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	warning, err := n.DefaultWebhookHandler.ValidateUpdate(ctx, oldObj, newObj)
	if warning != nil || err != nil {
		return warning, err
	}

	nx, ok := newObj.(*vshnv1.VSHNNextcloud)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNPostgreSQL object")
	}

	if err := validateFQDNs(nx.Spec.Parameters.Service.FQDN); err != nil {
		return nil, err
	}

	if nx.Spec.Parameters.AddOns.Office.Enabled {
		if err := validateFQDNs([]string{nx.Spec.Parameters.AddOns.Office.FQDN}); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func validateFQDNs(fqdns []string) error {
	for _, fqdn := range fqdns {
		if !valid.IsDNSName(fqdn) {
			return fmt.Errorf("FQDN %s is not a valid DNS name", fqdn)
		}
	}
	return nil
}
