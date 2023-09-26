package vshnminio

import (
	"context"
	"encoding/json"
	"strconv"

	xhelmbeta1 "github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

// DeployMinio will add deploy the objects to deploy minio
func DeployMinio(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	l := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNMinio{}
	err := iof.Observed.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't get composite", err)
	}

	l.Info("Creating namespace for minio instance")
	err = createObjectNamespace(ctx, comp, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot create object namespace", err)
	}

	l.Info("Creating helm release for minio instance")
	err = createObjectHelmRelease(ctx, comp, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot create object release", err)
	}

	l.Info("creating service observer")
	err = createServiceObserver(ctx, comp, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot create service observer", err)
	}

	l.Info("Get connection details from secret")
	err = getConnectionDetails(ctx, comp, iof)
	if err != nil {
		return runtime.NewWarning(ctx, "cannot get connection details")
	}

	return runtime.NewNormal()
}

// Create the namespace for the minio instance
func createObjectNamespace(ctx context.Context, comp *vshnv1.VSHNMinio, iof *runtime.Runtime) error {

	ns := &corev1.Namespace{

		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetInstanceNamespace(),
			Labels: map[string]string{
				"appcat.vshn.io/servicename":     "minio-distributed",
				"appcat.vshn.io/claim-namespace": comp.GetClaimNamespace(),
				"appuio.io/no-rbac-creation":     "true",
				"appuio.io/billing-name":         "appcat-minio"},
		},
	}

	return iof.Desired.PutIntoObject(ctx, ns, comp.Name+"-ns")
}

// Create the helm release for the minio instance
func createObjectHelmRelease(ctx context.Context, comp *vshnv1.VSHNMinio, iof *runtime.Runtime) error {

	plan := comp.Spec.Parameters.Size.GetPlan(iof.Config.Data["defaultPlan"])

	resouces, err := utils.FetchPlansFromConfig(ctx, iof, plan)
	if err != nil {
		return err
	}

	reqMem := comp.Spec.Parameters.Size.Requests.Memory
	reqCPU := comp.Spec.Parameters.Size.Requests.CPU
	mem := comp.Spec.Parameters.Size.Memory
	cpu := comp.Spec.Parameters.Size.CPU
	disk := comp.Spec.Parameters.Size.Disk

	if reqMem == "" {
		reqMem = resouces.MemoryRequests.String()
	}
	if reqCPU == "" {
		reqCPU = resouces.CPURequests.String()
	}
	if mem == "" {
		mem = resouces.MemoryLimits.String()
	}
	if cpu == "" {
		cpu = resouces.CPULimits.String()
	}
	if disk == "" {
		disk = resouces.Disk.String()
	}

	values := map[string]interface{}{
		"fullnameOverride": comp.GetName(),
		"mode":             comp.Spec.Parameters.Service.Mode,
		"replicas":         comp.Spec.Parameters.Instances,
		"networkPolicy": map[string]interface{}{
			"enabled": true,
		},
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{
				"memory": reqMem,
				"cpu":    reqCPU,
			},
			"limits": map[string]interface{}{
				"memory": mem,
				"cpu":    cpu,
			},
		},
		"persistence": map[string]interface{}{
			"size":         disk,
			"storageClass": comp.Spec.Parameters.StorageClass,
		},
		"securityContext": map[string]interface{}{
			"enabled": false,
		},
	}

	vb, err := json.Marshal(values)
	if err != nil {
		return err
	}

	r := &xhelmbeta1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
		Spec: xhelmbeta1.ReleaseSpec{
			ForProvider: xhelmbeta1.ReleaseParameters{
				Chart: xhelmbeta1.ChartSpec{
					Repository: iof.Config.Data["minioChartRepository"],
					Version:    iof.Config.Data["minioChartVersion"],
					Name:       "minio",
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
			ConnectionDetails: []xhelmbeta1.ConnectionDetail{
				{
					ObjectReference: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       comp.GetName(),
						Namespace:  comp.GetInstanceNamespace(),
						FieldPath:  "data.rootUser",
					},
					ToConnectionSecretKey: "AWS_ACCESS_KEY_ID",
				},
				{
					ObjectReference: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       comp.GetName(),
						Namespace:  comp.GetInstanceNamespace(),
						FieldPath:  "data.rootPassword",
					},
					ToConnectionSecretKey: "AWS_SECRET_ACCESS_KEY",
				},
			},
		},
	}

	cd := []v1alpha1.DerivedConnectionDetail{
		{
			Name:                    pointer.String("MINIO_USERNAME"),
			FromConnectionSecretKey: pointer.String("MINIO_USERNAME"),
			Type:                    v1alpha1.ConnectionDetailTypeFromConnectionSecretKey,
		},
		{
			Name:                    pointer.String("MINIO_PASSWORD"),
			FromConnectionSecretKey: pointer.String("MINIO_PASSWORD"),
			Type:                    v1alpha1.ConnectionDetailTypeFromConnectionSecretKey,
		},
	}

	return iof.Desired.PutWithResourceName(ctx, r, comp.Name+"-release", runtime.AddDerivedConnectionDetails(cd))
}

func createServiceObserver(ctx context.Context, comp *vshnv1.VSHNMinio, iof *runtime.Runtime) error {

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
	}

	return iof.Desired.PutIntoObserveOnlyObject(ctx, service, comp.Name+"-service-observer")
}

func getConnectionDetails(ctx context.Context, comp *vshnv1.VSHNMinio, iof *runtime.Runtime) error {

	service := &corev1.Service{}

	err := iof.Observed.GetFromObject(ctx, service, comp.Name+"-service-observer")
	if err != nil {
		return err
	}

	iof.Desired.PutCompositeConnectionDetail(ctx, v1alpha1.ExplicitConnectionDetail{
		Name:  "MINIO_URL",
		Value: "http://" + service.Spec.ClusterIP + ":" + strconv.Itoa(int(service.Spec.Ports[0].Port)),
	})

	return nil
}
