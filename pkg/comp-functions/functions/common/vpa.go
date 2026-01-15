package common

import (
	"context"
	"fmt"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AddVPAConfig creates a VPA CR and a VerticalPodAutoscaler that references it
func AddVPAConfig[T client.Object](ctx context.Context, obj T, svc *runtime.ServiceRuntime) *fnproto.Result {
	log := svc.Log

	err := svc.GetObservedComposite(obj)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	comp := obj.DeepCopyObject().(Composite)

	log.Info("Creating VPA configuration", "service", comp.GetName())

	// Create the VPA CR
	vpaCR := &appcatv1.VPA{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-vpa",
			Namespace: comp.GetInstanceNamespace(),
			Labels:    comp.GetLabels(),
		},
		Spec: appcatv1.VPASpec{
			Replicas: int32(comp.GetInstances()),
			Selector: fmt.Sprintf("cnpg.io/cluster=%s-cluster", comp.GetName()),
		},
	}

	err = svc.SetDesiredKubeObject(vpaCR, comp.GetName()+"-vpa-cr", runtime.KubeOptionAllowDeletion)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("could not set desired VPA CR: %w", err))
	}

	// Create the VerticalPodAutoscaler using unstructured
	vpaAutoscaler := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling.k8s.io/v1",
			"kind":       "VerticalPodAutoscaler",
			"metadata": map[string]interface{}{
				"name":      comp.GetName() + "-vpa-autoscaler",
				"namespace": comp.GetInstanceNamespace(),
				"labels":    comp.GetLabels(),
			},
			"spec": map[string]interface{}{
				"targetRef": map[string]interface{}{
					"apiVersion": "appcat.vshn.io/v1",
					"kind":       "VPA",
					"name":       comp.GetName() + "-vpa",
				},
				"updatePolicy": map[string]interface{}{
					"updateMode": "Auto",
				},
			},
		},
	}

	err = svc.SetDesiredKubeObject(vpaAutoscaler, comp.GetName()+"-vpa-autoscaler", runtime.KubeOptionAllowDeletion)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("could not set desired VPA autoscaler: %w", err))
	}

	log.Info("VPA configuration created", "vpaCR", vpaCR.Name, "vpaAutoscaler", vpaAutoscaler.GetName())

	return runtime.NewNormalResult("VPA and VerticalPodAutoscaler created")
}
