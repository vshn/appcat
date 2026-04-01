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
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

func DeployOpenBao(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	serviceName := comp.GetName()
	details := getServiceDetails(serviceName)

	// The Components Appcat defines plans (how much CPU, memory etc)
	// The XR object (created by user) defines only the plan.
	// Users cannot choose exact CPU and memory request. That is defined in Components Appcat.
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err).Error())
	}

	reqMem := cmp.Or(comp.Spec.Parameters.Size.Requests.Memory, resources.MemoryRequests.String())
	reqCPU := cmp.Or(comp.Spec.Parameters.Size.Requests.CPU, resources.CPURequests.String())
	mem := cmp.Or(comp.Spec.Parameters.Size.Memory, resources.MemoryLimits.String())
	cpu := cmp.Or(comp.Spec.Parameters.Size.CPU, resources.CPULimits.String())
	disk := cmp.Or(comp.Spec.Parameters.Size.Disk, resources.Disk.String())

	values := map[string]interface{}{
		"fullnameOverride": serviceName,
		"agent": map[string]any{
			"enabled": false,
		},
		"injector": map[string]any{
			"enabled": false,
		},
		"server": map[string]any{
			"ha": map[string]any{
				"enabled": true,
				"config":  "# Config provided via external file\n",
				"raft": map[string]any{
					"enabled": true,
					"config":  "# Config provided via external file\n",
				},
			},
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
			"extraArgs": fmt.Sprintf("-config=%s/%s", details.HclConfigMountPath, details.HclConfigFileName),
			"volumes": []map[string]any{
				{
					"name": details.HclVolumeName,
					"secret": map[string]any{
						"defaultMode": 420,
						"secretName":  details.HclConfigSecretName,
					},
				},
				{
					"name": details.TlsVolumeName,
					"secret": map[string]any{
						"defaultMode": 420,
						"secretName":  details.ServerCertSecretName,
					},
				},
			},
			"volumeMounts": []map[string]any{
				{
					"mountPath": details.HclConfigMountPath,
					"name":      details.HclVolumeName,
					"readOnly":  true,
				},
				{
					"mountPath": details.TlsCertsMountPath,
					"name":      details.TlsVolumeName,
					"readOnly":  true,
				},
			},
		},
	}

	vb, err := json.Marshal(values)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot marshal helm values: %w", err).Error())
	}

	r := &xhelmbeta1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
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
		return runtime.NewWarningResult(fmt.Errorf("cannot add %s connection details: %w", comp.Name+"-release", err).Error())
	}

	err = svc.SetDesiredComposedResourceWithName(r, comp.Name+"-release")
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot add %s composed resource: %w", comp.Name+"-release", err).Error())
	}

	return nil
}
