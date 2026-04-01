package sshgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	admissionv1 "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/mutate-gateway-networking-x-k8s-io-v1alpha1-xlistenerset,mutating=true,failurePolicy=fail,groups=gateway.networking.x-k8s.io,resources=xlistenersets,verbs=create,versions=v1alpha1,name=mxlistenerset.kb.io,admissionReviewVersions=v1,sideEffects=None

//+kubebuilder:rbac:groups=gateway.networking.x-k8s.io,resources=xlistenersets,verbs=get;list;watch
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create;get;list;watch;delete

// XListenerSetHandler handles mutating admission requests for XListenerSet resources.
// It allocates unique TCP ports for listeners that have port 0 (sentinel value).
type XListenerSetHandler struct {
	allocator *PortAllocator
	sharding  *GatewaySharding
	log       logr.Logger
	leaseNS   string
}

// SetupXListenerSetWebhookWithManager registers the XListenerSet mutating webhook.
// gatewayCapacity of 0 disables gateway sharding.
func SetupXListenerSetWebhookWithManager(mgr ctrl.Manager, portRangeStart, portRangeEnd int32, gatewayCapacity int, gatewayNS string, gatewayNames []string) error {
	allocator := NewPortAllocator(mgr.GetClient(), portRangeStart, portRangeEnd)
	handler := &XListenerSetHandler{
		allocator: allocator,
		log:       mgr.GetLogger().WithName("webhook").WithName("xlistenerset-sshgateway"),
		leaseNS:   gatewayNS,
	}

	if gatewayCapacity > 0 && len(gatewayNames) > 0 {
		gateways := make([]GatewayKey, len(gatewayNames))
		for i, name := range gatewayNames {
			gateways[i] = GatewayKey{Namespace: gatewayNS, Name: name}
		}
		handler.sharding = NewGatewaySharding(gateways, gatewayCapacity)
	}

	mgr.GetWebhookServer().Register("/mutate-gateway-networking-x-k8s-io-v1alpha1-xlistenerset",
		&webhook.Admission{Handler: handler})
	return nil
}

// Handle processes the admission request for XListenerSet resources.
// It works with map[string]any to preserve all fields through the marshal/unmarshal roundtrip.
func (h *XListenerSetHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create {
		return admission.Allowed("")
	}

	var obj map[string]any
	if err := json.Unmarshal(req.Object.Raw, &obj); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unmarshaling XListenerSet: %w", err))
	}

	metadata, _ := obj["metadata"].(map[string]any)
	name, _ := metadata["name"].(string)
	namespace, _ := metadata["namespace"].(string)

	l := h.log.WithValues("name", name, "namespace", namespace)

	spec, ok := obj["spec"].(map[string]any)
	if !ok {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("missing or invalid spec"))
	}

	items, err := h.allocator.listXListenerSets(ctx)
	if err != nil {
		l.Error(err, "Failed to list XListenerSets")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("listing XListenerSets: %w", err))
	}

	modified := false

	// if sharding enabled, check capacity and potentially reassign the parentRef.
	if h.sharding != nil {
		listenerCounts := h.allocator.extractListenerCounts(items)

		parentRef, _ := spec["parentRef"].(map[string]any)
		parentNs, _ := parentRef["namespace"].(string)
		parentName, _ := parentRef["name"].(string)

		if parentName == "" {
			return admission.Denied("spec.parentRef.name is required when gateway sharding is enabled")
		}

		currentRef := GatewayKey{
			Namespace: parentNs,
			Name:      parentName,
		}

		listeners, _ := spec["listeners"].([]any)
		selected, changed, err := h.sharding.SelectGateway(currentRef, len(listeners), listenerCounts)
		if err != nil {
			l.Info("All gateways full, denying admission", "error", err.Error())
			return admission.Denied(fmt.Sprintf("cannot place XListenerSet: %s", err))
		}

		if changed {
			l.Info("Reassigning gateway", "from", currentRef, "to", selected)
			parentRef["name"] = selected.Name
			parentRef["namespace"] = selected.Namespace
			modified = true
		}
	}

	usedPorts := h.allocator.extractUsedPorts(items)

	listeners, _ := spec["listeners"].([]any)
	for i, l := range listeners {
		listenerMap, ok := l.(map[string]any)
		if !ok {
			continue
		}
		port := utils.ToInt32(listenerMap["port"])
		if port == 0 {
			listenerName, _ := listenerMap["name"].(string)

			newPort, err := h.allocator.AllocatePort(ctx, usedPorts, h.leaseNS, name)
			if err != nil {
				h.log.Error(err, "Failed to allocate port", "listener", listenerName)
				return admission.Errored(http.StatusInternalServerError, fmt.Errorf("allocating port for listener %q: %w", listenerName, err))
			}

			h.log.Info("Allocated port", "listener", listenerName, "port", newPort)

			listenerMap["port"] = float64(newPort)
			listeners[i] = listenerMap
			modified = true
		}
	}

	if !modified {
		return admission.Allowed("")
	}

	patched, err := json.Marshal(obj)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("marshaling patched XListenerSet: %w", err))
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, patched)
}
