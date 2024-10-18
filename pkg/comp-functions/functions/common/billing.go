package common

import (
	"context"
	"fmt"
	v12 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"reflect"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"strings"
)

const billingLabel = "appcat.io/billing"

// InjectBillingLabelToService adds billing label to a service (StatefulSet or Deployment).
// It uses a kube Object to achieve post provisioning labelling
func InjectBillingLabelToService(ctx context.Context, svc *runtime.ServiceRuntime, comp InfoGetter) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Enabling billing for service", "service", comp.GetName())

	s := comp.GetWorkloadPodTemplateLabelsManager()
	s.SetName(comp.GetWorkloadName())
	s.SetNamespace(comp.GetInstanceNamespace())
	kubeName := comp.GetName() + "-" + getType(s)

	_ = svc.GetObservedKubeObject(s, kubeName)
	mp := v12.ManagementPolicies{v12.ManagementActionObserve}
	labels := s.GetPodTemplateLabels()
	_, exists := labels[billingLabel]
	if !s.GetCreationTimestamp().Time.IsZero() {
		if !exists {
			labels[billingLabel] = "true"
			s.SetPodTemplateLabels(labels)
			mp = append(mp, v12.ManagementActionCreate, v12.ManagementActionUpdate)
		}
	}

	err := svc.SetDesiredKubeObject(s.GetObject(), kubeName, func(obj *xkube.Object) {
		obj.Spec.ManagementPolicies = mp
	})

	if err != nil && !exists {
		runtime.NewWarningResult(fmt.Sprintf("cannot add billing to service object %s", s.GetName()))
	}

	return runtime.NewNormalResult("billing enabled")
}

func getType(myvar interface{}) (res string) {
	return strings.ToLower(reflect.TypeOf(myvar).Elem().Field(0).Name)
}
