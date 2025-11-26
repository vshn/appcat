package vshnopenbao

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

// DeployConfigMap creates a ConfigMap with a hello world message and today's date
func DeployOpenBao(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {

	serviceName := comp.GetServiceName()
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	svc.Log.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, serviceName, comp.GetName()+"-ns", svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot bootstrap instance namespace: %s", err))
	}

	svc.Log.Info("Creating helm release for OpenBao instance")
	err = createObjectHelmRelease(ctx, comp, svc)
	if err != nil {
		err = fmt.Errorf("cannot create helm release: %w", err)
		return runtime.NewFatalResult(err)
	}

	return nil
}
func createObjectHelmRelease(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) error {

	// The Components Appcat defines plans (how much CPU, memory etc)
	// The XR object (created by user) defines only the plan.
	// Users cannot choose exact CPU and memory request. That is defined in Components Appcat.
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resouces, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return err
	}

	reqMem := cmp.Or(comp.Spec.Parameters.Size.Requests.Memory, resouces.MemoryRequests.String())
	reqCPU := cmp.Or(comp.Spec.Parameters.Size.Requests.CPU, resouces.CPURequests.String())
	mem := cmp.Or(comp.Spec.Parameters.Size.Memory, resouces.MemoryLimits.String())
	cpu := cmp.Or(comp.Spec.Parameters.Size.CPU, resouces.CPULimits.String())
	disk := cmp.Or(comp.Spec.Parameters.Size.Disk, resouces.Disk.String())

	values := map[string]interface{}{
		"fullnameOverride": comp.GetName(),
		"agent": map[string]any{
			"enabled": false,
		},
		"injector": map[string]any{
			"enabled": false,
		},
		"server": map[string]any{
			"authDelegator": map[string]any{
				"enabled": false,
			},
			"resources": map[string]any{
				"requests": map[string]any{
					"memory": reqMem,
					"cpu":    reqCPU,
				},
				"limits": map[string]any{
					"memory": mem,
					"cpu":    cpu,
				},
			},
			"dataStorage": map[string]any{
				"enabled": true,
				"size":    disk,
			},
		},
	}

	vb, err := json.Marshal(values)
	if err != nil {
		err = fmt.Errorf("cannot marshal helm values: %w", err)
		return err
	}

	r := &xhelmbeta1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
		Spec: xhelmbeta1.ReleaseSpec{
			ForProvider: xhelmbeta1.ReleaseParameters{
				Chart: xhelmbeta1.ChartSpec{
					Repository: svc.Config.Data["chartRepository"],
					Version:    svc.Config.Data["chartVersion"],
					Name:       "openbao",
				},
				Namespace: comp.GetInstanceNamespace(),
				ValuesSpec: xhelmbeta1.ValuesSpec{
					Values: k8sruntime.RawExtension{
						Raw: vb,
					},
				},
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "helm",
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      comp.GetName() + "-connection",
					Namespace: comp.GetInstanceNamespace(),
				},
			},
			ConnectionDetails: []xhelmbeta1.ConnectionDetail{},
		},
	}

	err = svc.AddObservedConnectionDetails(comp.Name + "-release")
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResourceWithName(r, comp.Name+"-release")
}
