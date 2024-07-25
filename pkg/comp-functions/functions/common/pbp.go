package common

import (
	"context"
	"fmt"
	"reflect"

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
		pdb, ok := comp.(PDB)
		if !ok {
			return runtime.NewFatalResult(fmt.Errorf("type %s doesn't implement PDB interface", reflect.TypeOf(comp).String()))
		}

		if pdb.GetInstances() < 2 {
			return runtime.NewNormalResult("Not HA, no pdb needed")
		}
		log.Info("HA detected, adding pdb", "service", comp.GetName())

		x := &pdbv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      comp.GetName() + "-pdb",
				Namespace: pdb.GetInstanceNamespace(),
			},
		}
		min := intstr.IntOrString{}
		if pdb.GetInstances() == 2 {
			// 1 working instance is still better than nothing
			min.IntVal = 1
		} else {
			// no matter if there are 3,5,10 instances, 2 should be the minimum to keep HA working
			min.IntVal = 2
		}

		x.Spec.MinAvailable = ptr.To(min)
		x.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: pdb.GetPDBLabels(),
		}

		err = svc.SetDesiredKubeObject(x, comp.GetName()+"-pdb")
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("could not set desired kube compect: %w", err))
		}

		return runtime.NewNormalResult("PDB created")
	}
}
