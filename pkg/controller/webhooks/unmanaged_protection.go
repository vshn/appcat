package webhooks

import (
	"context"
	"fmt"
	"net/http"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// UnamanagedHandler implements the admission UnmanagedHandler for unmanaged objects (PVC, services, etc).
type UnamanagedHandler struct {
	client             client.Client
	controlPlaneClient client.Client
	log                logr.Logger
}

// SetupWebhookWithManager sets up the webhook with the manager.
func SetupUnmanagedProtectionWebhookWithManager(mgr ctrl.Manager) error {
	cpClient, err := getControlPlaneClient(mgr)
	if err != nil {
		return err
	}

	mgr.GetWebhookServer().Register("/unmanaged-deletion-protection",
		&webhook.Admission{Handler: &UnamanagedHandler{
			client:             mgr.GetClient(),
			controlPlaneClient: cpClient,
			log:                mgr.GetLogger().WithName("webhook").WithName("unmanaged"),
		}})
	return nil
}

func (u *UnamanagedHandler) Handle(ctx context.Context, request admission.Request) admission.Response {

	switch request.Operation {
	case admissionv1.Delete:
		oldObj := &unstructured.Unstructured{}
		if err := oldObj.UnmarshalJSON(request.OldObject.Raw); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		l := u.log.WithValues("object", oldObj.GetName(), "namespace", oldObj.GetNamespace(), "GVK", oldObj.GetObjectKind().GroupVersionKind().String())

		compInfo, err := checkUnmanagedObject(ctx, oldObj, u.client, u.controlPlaneClient, l)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if compInfo.Exists {
			l.Info("Blocking deletion of object", "parent", compInfo.Name)
			return admission.Denied(fmt.Sprintf(protectedMessage, oldObj.GetObjectKind().GroupVersionKind().Kind, compInfo.Name))
		}

		l.Info("Allowing deletion of object", "name", compInfo.Name)

		return admission.Allowed("")
	default:
		return admission.Errored(http.StatusBadRequest, errors.Errorf("unexpected operation: %s", request.Operation))
	}

}
