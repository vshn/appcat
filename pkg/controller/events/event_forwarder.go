package events

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type EventHandler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (e *EventHandler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	event := &corev1.Event{}
	err := e.Get(ctx, req.NamespacedName, event)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, most likely it has already been deleted
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the event - requeue the request
		log.Error(err, "Can't read the event", "event", event)
		return ctrl.Result{}, err
	}
	// We might get events without a name. We don't care about those.
	// Return and don't requeue
	if strings.HasPrefix(event.Name, ".") {
		return ctrl.Result{}, nil
	}

	// We don't care about events of events without a `Kind` or `apiVersion`
	// Return and don't requeue
	if event.InvolvedObject.Kind == "" || event.InvolvedObject.APIVersion == "" {
		return ctrl.Result{}, nil
	}

	// Get the object the event is about
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.FromAPIVersionAndKind(event.InvolvedObject.APIVersion, event.InvolvedObject.Kind))
	if err := e.Get(ctx, types.NamespacedName{Name: event.InvolvedObject.Name, Namespace: event.InvolvedObject.Namespace}, obj); err != nil {
		if errors.IsNotFound(err) {
			// InvolvedObject has already been deleted.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		if errors.IsForbidden(err) {
			// We don't care for events for objects where the controller doesn't have access to.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request
		log.Error(err, "Can't read object", "object", obj)
		return ctrl.Result{}, err
	}
	// Check if the object has the annotation to forward events
	annotationEventForward := obj.GetAnnotations()["appcat.vshn.io/forward-events-to"]
	if annotationEventForward != "" {
		log.Info("Forwarding event", "event", event.Name, "to", annotationEventForward)
		forwardingObject := strings.Split(annotationEventForward, "/")
		apiVersion := forwardingObject[0] + "/" + forwardingObject[1]
		kind := forwardingObject[2]
		ns := forwardingObject[3]
		claimName := forwardingObject[4]

		// Get the claim to forward the event to
		claim := &unstructured.Unstructured{}
		claim.SetGroupVersionKind(schema.FromAPIVersionAndKind(apiVersion, kind))
		if err := e.Get(ctx, types.NamespacedName{Name: claimName, Namespace: ns}, claim); err != nil {
			return ctrl.Result{}, err
		}
		nev := &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      event.Name,
				Namespace: ns,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:            kind,
				Namespace:       ns,
				Name:            claimName,
				APIVersion:      apiVersion,
				ResourceVersion: claim.GetResourceVersion(),
				UID:             claim.GetUID(),
			},
		}

		// Create or patch the new forwarded event
		_, err := controllerutil.CreateOrPatch(ctx, e.Client, nev, func() error {
			nev.Count = event.Count
			nev.FirstTimestamp = event.FirstTimestamp
			nev.LastTimestamp = event.LastTimestamp
			nev.Message = event.Message
			nev.Reason = event.Reason
			nev.ReportingController = event.ReportingController
			nev.ReportingInstance = event.ReportingInstance
			nev.Type = event.Type
			nev.Source = event.Source
			return nil
		})

		if err != nil {
			log.Error(err, "Can't create or patch the event")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func ignoreEvents() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Ignore events from namespaces other than "default" or vshn appcat namespaces
			ns := e.Object.GetNamespace()
			event, ok := e.Object.(*corev1.Event)
			if !ok {
				return false
			}
			if event.Type == "Normal" {
				return false
			}
			return (ns == "default" || strings.HasPrefix(ns, "vshn-"))
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore events from namespaces other than "default" or vshn appcat namespaces
			ns := e.ObjectNew.GetNamespace()
			event, ok := e.ObjectNew.(*corev1.Event)
			if !ok {
				return false
			}
			if event.Type == "Normal" {
				return false
			}
			return (ns == "default" || strings.HasPrefix(ns, "vshn-"))
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// We ignore deleted events
			return false
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (e *EventHandler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Event{}).
		WithEventFilter(ignoreEvents()).
		Complete(e)
}
