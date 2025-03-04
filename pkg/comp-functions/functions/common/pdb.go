package common

import (
	"context"
	"fmt"

	pdbv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func AddPDBSettings[T client.Object](ctx context.Context, obj T, svc *runtime.ServiceRuntime) *fnproto.Result {

	log := svc.Log

	err := svc.GetObservedComposite(obj)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	comp := obj.DeepCopyObject().(Composite)

	log.Info("Checking if PDB is needed", "service", comp.GetName(), "instances", comp.GetInstances())

	if comp.GetInstances() < 2 {
		return runtime.NewNormalResult("Not HA, no pdb needed")
	}
	log.Info("HA detected, adding pdb", "service", comp.GetName())

	min := intstr.IntOrString{StrVal: "50%", Type: intstr.String}

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

	err = svc.SetDesiredKubeObject(x, comp.GetName()+"-pdb", runtime.KubeOptionAllowDeletion)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("could not set desired kube compect: %w", err))
	}

	return runtime.NewNormalResult("PDB created")
}
