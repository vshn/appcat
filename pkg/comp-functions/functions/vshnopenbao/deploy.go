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
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	serviceName := comp.GetName()

	// The Components Appcat defines plans (how much CPU, memory etc)
	// The XR object (created by user) defines only the plan.
	// Users cannot choose exact CPU and memory request. That is defined in Components Appcat.
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err).Error())
	}

	reqMem := cmp.Or(comp.Spec.Parameters.Size.MemoryRequests, resources.MemoryRequests.String())
	reqCPU := cmp.Or(comp.Spec.Parameters.Size.CPURequests, resources.CPURequests.String())
	mem := cmp.Or(comp.Spec.Parameters.Size.MemoryLimits, resources.MemoryLimits.String())
	cpu := cmp.Or(comp.Spec.Parameters.Size.CPULimits, resources.CPULimits.String())
	disk := cmp.Or(comp.Spec.Parameters.Size.Disk, resources.Disk.String())

	values := map[string]interface{}{
		"fullnameOverride": serviceName,
		"agent": map[string]any{
			"enabled": false,
		},
		"global": map[string]interface{}{
			"tlsDisable": false,
		},
		"injector": map[string]any{
			"enabled": false,
		},
		"server": map[string]any{
			"rbac": map[string]any{
				"create": false,
			},
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
			"extraArgs": fmt.Sprintf("-config=%s/%s", hclConfigMountPath, hclConfigFileName),
			"volumes": []map[string]any{
				{
					"name": hclConfigVolumeName,
					"secret": map[string]any{
						"defaultMode": 420,
						"secretName":  serviceName + hclConfigSecretSuffix,
					},
				},
				{
					"name": hclConfigTlsVolumeName,
					"secret": map[string]any{
						"defaultMode": 420,
						"secretName":  serverCertSecretName,
					},
				},
			},
			"volumeMounts": []map[string]any{
				{
					"mountPath": hclConfigMountPath,
					"name":      hclConfigVolumeName,
					"readOnly":  true,
				},
				{
					"mountPath": hclConfigTlsCertsMountPath,
					"name":      hclConfigTlsVolumeName,
					"readOnly":  true,
				},
			},
		},
	}

	if comp.Spec.Parameters.OpenBao.Init.RunInitJob {
		server := values["server"].(map[string]any)
		server["extraContainers"] = []map[string]any{
			{
				"name":    "init-openbao",
				"image":   svc.Config.Data["kubectl_image"],
				"command": []string{"bash", "-c"},
				"args":    []string{initClusterScript},
				"volumeMounts": []map[string]any{
					{
						"name":      hclConfigTlsVolumeName,
						"mountPath": "/tls",
						"readOnly":  true,
					},
				},
				"env": []map[string]any{
					// POD_NAME must come first — VAULT_INIT_ADDR below references $(POD_NAME)
					// via Kubernetes env-var substitution so it resolves to the pod's headless DNS.
					{"name": "POD_NAME", "valueFrom": map[string]any{
						"fieldRef": map[string]any{"fieldPath": "metadata.name"},
					}},
					{"name": "VAULT_ADDR", "value": fmt.Sprintf("https://%s:8200", serviceName)},
					// Headless pod address used for init API calls. The regular service only routes
					// to ready pods, but pod-0 is not ready until after initialization.
					{"name": "VAULT_INIT_ADDR", "value": fmt.Sprintf("https://$(POD_NAME).%s-internal.%s.svc.cluster.local:8200", serviceName, comp.GetInstanceNamespace())},
					{"name": "NAMESPACE", "value": comp.GetInstanceNamespace()},
					{"name": "ROOT_TOKEN_SECRET_NAME", "value": serviceName + initOutputSecretSuffix},
					{"name": "UNSEAL_KEYS_SECRET_NAME", "value": serviceName + unsealKeysSecretSuffix},
					{"name": "SECRET_SHARES", "value": fmt.Sprintf("%d", comp.Spec.Parameters.OpenBao.Init.SecretShares)},
					{"name": "SECRET_THRESHOLD", "value": fmt.Sprintf("%d", comp.Spec.Parameters.OpenBao.Init.SecretThreshold)},
				},
			},
		}
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
					// TODO set default values. Test what happens if not set
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
					Name:      serviceName + "-connection",
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
