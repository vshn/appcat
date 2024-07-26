package common

import (
	"context"
	"fmt"

	pdbv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ptr "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func AddPDBSettings(comp client.Object) func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {
	return func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {

		log := svc.Log

		err := svc.GetObservedComposite(comp)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
		}
		infoGetter, ok := comp.(InfoGetter)
		if !ok {
			return runtime.NewFatalResult(fmt.Errorf("could not cast to InfoGetter"))
		}

		log.Info("Checking if PDB is needed", "service", infoGetter.GetName(), "instances", infoGetter.GetInstances())

		if infoGetter.GetInstances() < 2 {
			return runtime.NewNormalResult("Not HA, no pdb needed")
		}
		log.Info("HA detected, adding pdb", "service", infoGetter.GetName())

		x := &pdbv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      infoGetter.GetName() + "-pdb",
				Namespace: infoGetter.GetInstanceNamespace(),
			},
		}
		min := intstr.IntOrString{StrVal: "50%"}

		x.Spec.MinAvailable = ptr.To(min)
		x.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: infoGetter.GetPDBLabels(),
		}

		err = svc.SetDesiredKubeObject(x, infoGetter.GetName()+"-pdb")
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("could not set desired kube compect: %w", err))
		}

		return runtime.NewNormalResult("PDB created")
	}
}
