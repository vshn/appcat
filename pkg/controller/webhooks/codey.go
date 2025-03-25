package webhooks

import (
	"context"
	"fmt"
	"strings"

	codey "github.com/vshn/appcat/v4/apis/codey"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	field "k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=create;update,path=/validate-codey-io-v1-codeyinstance,mutating=false,failurePolicy=fail,groups=codey.io,resources=codeyinstances,versions=v1,name=codeyinstance.codey.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list
//+kubebuilder:rbac:groups=codey.io,resources=codeyinstances,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=codey.io,resources=codeyinstances/status,verbs=get;list;watch;patch;update

const (
	codeyUrlSuffix = ".app.codey.ch"
)

var (
	codeyGK = schema.GroupKind{Group: "codey.io", Kind: "CodeyInstance"}
	codeyGR = schema.GroupResource{Group: codeyGK.Group, Resource: "codeyinstance"}
)

var _ webhook.CustomValidator = &CodeyInstanceWebhookHandler{}

type CodeyInstanceWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupCodeyInstanceWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupCodeyInstanceWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&codey.CodeyInstance{}).
		WithValidator(&CodeyInstanceWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("codey"),
				withQuota,
				&codey.CodeyInstance{},
				"codey",
				codeyGK,
				codeyGR,
			),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (n *CodeyInstanceWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	warning, err := n.DefaultWebhookHandler.ValidateCreate(ctx, obj)
	if warning != nil || err != nil {
		return warning, err
	}

	codey, ok := obj.(*codey.CodeyInstance)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid CodeyInstance object")
	}

	codeyFqdn := codey.ObjectMeta.Name + codeyUrlSuffix
	if err := isCodeyFqdnUnique(codeyFqdn, n.client); err != nil {
		return nil, fmt.Errorf("failed FQDN validation: %v", err)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *CodeyInstanceWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	_, ok := oldObj.(*codey.CodeyInstance)
	if !ok {
		return nil, fmt.Errorf("not a valid CodeyInstance object")
	}
	newCodeyInstance, ok := newObj.(*codey.CodeyInstance)
	if !ok {
		return nil, fmt.Errorf("not a valid CodeyInstance object")
	}

	codeyFqdn := newCodeyInstance.ObjectMeta.Name + codeyUrlSuffix
	if err := isCodeyFqdnUnique(codeyFqdn, p.client); err != nil {
		return nil, fmt.Errorf("failed FQDN validation: %v", err)
	}

	return p.DefaultWebhookHandler.ValidateUpdate(ctx, oldObj, newObj)
}

// Checks if a given FQDN is already in use by some CodeyInstance in the cluster
func isCodeyFqdnUnique(fqdn string, cl client.Client) error {
	ingressList := &netv1.IngressList{}
	err := cl.List(context.TODO(), ingressList, client.MatchingLabels{
		"appcat.vshn.io/ownerkind": "XVSHNForgejo",
	})
	if err != nil {
		return fmt.Errorf("failed listing ingresses: %v", err)
	}

	for _, ingress := range ingressList.Items {
		for _, rule := range ingress.Spec.Rules {
			if rule.Host == fqdn {
				return field.Invalid(
					field.NewPath("metadata", "name"),
					strings.Split(fqdn, ".")[0],
					fmt.Sprintf("produces a codey FQDN (%s) that is already in use", fqdn),
				)
			}
		}
	}

	return nil
}
