package vshnminio

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"strconv"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	crossplane "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	SLIBucketName = "vshn-test-bucket-for-sli"
)

// TODO refactor the code and use common.BootstrapInstanceNs()

// DeployMinio will add deploy the objects to deploy minio
func DeployMinio(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	l := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNMinio{}
	serviceName := comp.GetServiceName()
	err := svc.GetObservedComposite(comp)
	if err != nil {
		err = fmt.Errorf("cannot get observed composite: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Creating namespace for minio instance")
	err = createObjectNamespace(ctx, comp, svc)
	if err != nil {
		err = fmt.Errorf("cannot create minio namespace: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Creating helm release for minio instance")
	err = createObjectHelmRelease(ctx, comp, svc)
	if err != nil {
		err = fmt.Errorf("cannot create helm release: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Creating service observer")
	err = createServiceObserver(ctx, comp, svc)
	if err != nil {
		err = fmt.Errorf("cannot create service observer: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Creating service monitor")
	err = createServiceMonitor(ctx, comp, svc)
	if err != nil {
		err = fmt.Errorf("cannot create service monitor; %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Get connection details from secret")
	err = getConnectionDetails(ctx, comp, svc)
	if err != nil {
		if err == runtime.ErrNotFound {
			return runtime.NewNormalResult("skipping sli bucket, connectiondetails not yet available")
		}
		err = fmt.Errorf("cannot get connection details: %w", err)
		return runtime.NewFatalResult(fmt.Errorf("cannot get connection details: %w", err))
	}

	l.Info("Creating namespace policy to allow access to " + serviceName + " instance")
	err = common.CreateNetworkPolicy(comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create namespace policy  for %s instance: %w", serviceName, err))
	}

	l.Info("Starting vshn-test-bucket-for-sli creation")
	if err := createSliBucket(ctx, comp, comp.Labels["crossplane.io/claim-name"], svc); err != nil {
		l.Info("Failed to create SLI bucket")
		return runtime.NewFatalResult(fmt.Errorf("can't create SliBucket: %w", err))
	}
	return nil
}

// Create the namespace for the minio instance
func createObjectNamespace(ctx context.Context, comp *vshnv1.VSHNMinio, svc *runtime.ServiceRuntime) error {

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

	return svc.SetDesiredKubeObject(ns, comp.Name+"-ns")
}

// Create the helm release for the minio instance
func createObjectHelmRelease(ctx context.Context, comp *vshnv1.VSHNMinio, svc *runtime.ServiceRuntime) error {

	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resouces, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
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
		"deploymentUpdate": map[string]interface{}{
			"type": "Recreate",
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
					Repository: svc.Config.Data["minioChartRepository"],
					Version:    svc.Config.Data["minioChartVersion"],
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

	err = svc.AddObservedConnectionDetails(comp.Name + "-release")
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResourceWithName(r, comp.Name+"-release")
}

func createServiceObserver(ctx context.Context, comp *vshnv1.VSHNMinio, svc *runtime.ServiceRuntime) error {

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
	}

	return svc.SetDesiredKubeObserveObject(service, comp.Name+"-service-observer")
}

func getConnectionDetails(ctx context.Context, comp *vshnv1.VSHNMinio, svc *runtime.ServiceRuntime) error {

	service := &corev1.Service{}

	err := svc.GetObservedKubeObject(service, comp.Name+"-service-observer")
	if err != nil {
		if err == runtime.ErrNotFound {
			return err
		}
		err = fmt.Errorf("cannot get observed connectiondetails: %w", err)
		return err
	}

	minioURL := fmt.Sprintf("http://%s:%s", service.Spec.ClusterIP, strconv.Itoa(int(service.Spec.Ports[0].Port)))

	svc.SetConnectionDetail("MINIO_URL", []byte(minioURL))

	return nil
}

func createServiceMonitor(ctx context.Context, comp *vshnv1.VSHNMinio, svc *runtime.ServiceRuntime) error {

	sm := &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: promv1.ServiceMonitorSpec{
			Endpoints: []promv1.Endpoint{
				{
					Port:   "http",
					Scheme: "http",
					Path:   "/minio/v2/metrics/node",
				},
				{
					Port:   "http",
					Scheme: "http",
					Path:   "/minio/v2/metrics/cluster",
				},
				{
					Port:   "http",
					Scheme: "http",
					Path:   "/minio/v2/metrics/bucket",
				},
				{
					Port:   "http",
					Scheme: "http",
					Path:   "/minio/v2/metrics/resource",
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":        "minio",
					"monitoring": "true",
					"release":    comp.GetName(),
				},
			},
			NamespaceSelector: promv1.NamespaceSelector{
				MatchNames: []string{
					comp.GetInstanceNamespace(),
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(sm, comp.Name+"-service-monitor")
}

func createSliBucket(ctx context.Context, comp *vshnv1.VSHNMinio, xminioName string, svc *runtime.ServiceRuntime) error {
	obj := &v1.ObjectBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SLIBucketName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: v1.ObjectBucketSpec{
			Parameters: v1.ObjectBucketParameters{
				BucketName: SLIBucketName,
				Region:     "us-east-1",
			},
			WriteConnectionSecretToRef: v1.LocalObjectReference{
				Name:      SLIBucketName,
				Namespace: comp.GetInstanceNamespace(),
			},
			CompositionReference: crossplane.CompositionReference{
				Name: fmt.Sprintf("%s.objectbuckets.appcat.vshn.io", xminioName),
			},
		},
	}
	return svc.SetDesiredKubeObject(obj, comp.Name+"-vshn-test-bucket-for-sli")
}
