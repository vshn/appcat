package common

import (
	"context"
	"fmt"
	"reflect"

	pdbv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ptr "k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func AddPDBSettings(obj client.Object) func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {
	return func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {

		log := controllerruntime.LoggerFrom(ctx)
		log.Info("Checking if alerting references are set")

		err := svc.GetObservedComposite(obj)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
		}
		pdb, ok := obj.(PDB)
		if !ok {
			return runtime.NewWarningResult(fmt.Sprintf("Type %s doesn't implement PDB interface", reflect.TypeOf(obj).String()))
		}

		if pdb.GetInstances() < 2 {
			return runtime.NewNormalResult("Not HA, no pdb needed")
		}

		x := pdbv1.PodDisruptionBudget{}
		min := intstr.IntOrString{}

		switch pdb.GetInstances() {
		case 2:
			min.IntVal = 1
		case 3:
			min.IntVal = 2
		default:
			return runtime.NewWarningResult(fmt.Sprintf("Instances %d not supported", pdb.GetInstances()))
		}

		x.Spec.MinAvailable = ptr.To(min)
		x.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": pdb.GetInstanceNamespace(),
			},
		}

		return nil
	}
}
