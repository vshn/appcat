package common

import (
	"context"
	"fmt"

	pdbv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func AddPDBSettings(comp Composite) func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {
	return func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {

		log := svc.Log

		err := svc.GetObservedComposite(comp)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
		}

		log.Info("Checking if PDB is needed", "service", comp.GetName(), "instances", comp.GetInstances())

		if comp.GetInstances() < 2 {
			return runtime.NewNormalResult("Not HA, no pdb needed")
		}
		log.Info("HA detected, adding pdb", "service", comp.GetName())

		min := intstr.IntOrString{IntVal: int32(comp.GetInstances()) / 2}

		x := &pdbv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      comp.GetName() + "-pdb",
				Namespace: comp.GetInstanceNamespace(),
			},
			Spec: pdbv1.PodDisruptionBudgetSpec{
				MinAvailable: &min,
				Selector: &metav1.LabelSelector{
					MatchLabels: comp.GetPDBLabels(),
				},
			},
		}

		err = svc.SetDesiredKubeObject(x, comp.GetName()+"-pdb")
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("could not set desired kube compect: %w", err))
		}

		return runtime.NewNormalResult("PDB created")
	}
}
