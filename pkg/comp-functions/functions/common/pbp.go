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
	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func AddPDBSettings(comp client.Object) func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {
	return func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {

		log := svc.Log

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
		log.Info("HA detected, adding pdb", "service", obj.GetName())

		x := &pdbv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.GetName() + "-pdb",
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

		emptyMap := map[string]string{}

		// to add new case, simply print labels, for example `kubectl -n vshn-mariadb-something-123sa get pod mariadb-something-123sa-0 -o json | jq .metadata.labels` and add the label to the map
		switch obj.(type) {
		case *v1.VSHNKeycloak:
			emptyMap["app.kubernetes.io/name"] = obj.GetLabels()["crossplane.io/composite"]
		case *v1.VSHNPostgreSQL:
			emptyMap["stackgres.io/cluster-name"] = obj.GetLabels()["crossplane.io/composite"] + "-pg"
		default:
			return runtime.NewWarningResult(fmt.Sprintf("Type %s doesn't implement PDB interface", reflect.TypeOf(obj).String()))
		}

		x.Spec.MinAvailable = ptr.To(min)
		x.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: emptyMap,
		}

		err = svc.SetDesiredKubeObject(x, obj.GetName()+"-pdb")
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("could not set desired kube object: %w", err))
		}

		return runtime.NewNormalResult("PDB created")
	}
}
