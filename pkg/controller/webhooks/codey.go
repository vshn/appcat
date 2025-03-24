package webhooks

import (
	"context"
	"fmt"

	codey "github.com/vshn/appcat/v4/apis/codey"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
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
	if err := isCodeyFqdnUnique(codeyFqdn); err != nil {
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
	if err := isCodeyFqdnUnique(codeyFqdn); err != nil {
		return nil, fmt.Errorf("failed FQDN validation: %v", err)
	}

	return p.DefaultWebhookHandler.ValidateUpdate(ctx, oldObj, newObj)
}

// Checks if a given FQDN is already in use by some CodeyInstance in the cluster
func isCodeyFqdnUnique(fqdn string) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	namespaceList, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "appcat.vshn.io/ownerkind=XVSHNForgejo",
	})
	if err != nil {
		return fmt.Errorf("failed listing namespaces: %v", err)
	}

	for _, namespace := range namespaceList.Items {
		ingressList, err := clientset.NetworkingV1().Ingresses(namespace.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed listing ingresses in namespace %s: %v", namespace.Name, err)
		}

		for _, ingress := range ingressList.Items {
			for _, rule := range ingress.Spec.Rules {
				if rule.Host == fqdn {
					return fmt.Errorf("codey FQDN '%s' already present in namespace '%s'", fqdn, namespace.Name)
				}
			}
		}
	}

	return nil
}
