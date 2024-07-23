package webhooks

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnkeycloak,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnkeycloaks,versions=v1,name=vshnkeycloak.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnkeycloaks,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnkeycloaks/status,verbs=get;list;watch;patch;update

var (
	keycloakGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNKeycloak"}
	keycloakGR = schema.GroupResource{Group: keycloakGK.Group, Resource: "vshnkeycloak"}
)

var _ webhook.CustomValidator = &KeycloakWebhookHandler{}

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
