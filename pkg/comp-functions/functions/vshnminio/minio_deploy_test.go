package vshnminio

import (
	"context"
	"encoding/json"
	"testing"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

func TestMinioDeploy(t *testing.T) {

	svc, comp := getMinioComp(t)

	ctx := context.TODO()

	rootUser := "minio"
	rootPassword := "minio123"
	minioHost := "http://10.0.0.1:9000"

	assert.Nil(t, DeployMinio(ctx, &vshnv1.VSHNMinio{}, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetObservedKubeObject(ns, comp.Name+"-ns"))

	r := &xhelmbeta1.Release{}
	assert.NoError(t, svc.GetObservedComposedResource(r, comp.Name+"-release"))

	service := &corev1.Service{}
	assert.NoError(t, svc.GetObservedKubeObject(service, comp.Name+"-service-observer"))

	objBuck := &v1.ObjectBucket{}
	assert.NoError(t, svc.GetDesiredKubeObject(objBuck, comp.Name+"-vshn-test-bucket-for-sli"))

	cd := svc.GetConnectionDetails()
	assert.Equal(t, rootUser, string(cd["AWS_ACCESS_KEY_ID"]))
	assert.Equal(t, rootPassword, string(cd["AWS_SECRET_ACCESS_KEY"]))
	assert.Equal(t, minioHost, string(cd["MINIO_URL"]))

	sm := &promv1.ServiceMonitor{}
	assert.NoError(t, svc.GetDesiredKubeObject(sm, comp.Name+"-service-monitor"))
	assert.Equal(t, "/minio/v2/metrics/node", sm.Spec.Endpoints[0].Path)
	assert.Equal(t, "/minio/v2/metrics/cluster", sm.Spec.Endpoints[1].Path)
	assert.Equal(t, "/minio/v2/metrics/bucket", sm.Spec.Endpoints[2].Path)
	assert.Equal(t, "/minio/v2/metrics/resource", sm.Spec.Endpoints[3].Path)

	np := &netv1.NetworkPolicy{}
	assert.NoError(t, svc.GetDesiredKubeObject(np, comp.Name+"-netpol"))

}

func TestMinioDeploy_ImageRegistry(t *testing.T) {
	tests := []struct {
		name           string
		registry       string
		prefix         string
		wantImage      string
		wantMcImage    string
		wantNoOverride bool
	}{
		{
			name:           "GivenNoConfig_ThenChartDefaults",
			wantNoOverride: true,
		},
		{
			name:        "GivenRegistry_ThenDefaultPrefix",
			registry:    "registry.example.com/",
			wantImage:   "registry.example.com/minio/minio",
			wantMcImage: "registry.example.com/minio/mc",
		},
		{
			name:        "GivenPrefix_ThenDefaultRegistry",
			prefix:      "mirror/minio",
			wantImage:   "quay.io/mirror/minio/minio",
			wantMcImage: "quay.io/mirror/minio/mc",
		},
		{
			name:        "GivenRegistryAndPrefix_ThenBoth",
			registry:    "registry.example.com",
			prefix:      "customer",
			wantImage:   "registry.example.com/customer/minio",
			wantMcImage: "registry.example.com/customer/mc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, comp := getMinioComp(t)
			svc.Config.Data["imageRegistry"] = tt.registry
			svc.Config.Data["imageRepositoryPrefix"] = tt.prefix

			assert.Nil(t, DeployMinio(context.TODO(), &vshnv1.VSHNMinio{}, svc))

			r := &xhelmbeta1.Release{}
			assert.NoError(t, svc.GetDesiredComposedResourceByName(r, comp.Name+"-release"))

			values := map[string]interface{}{}
			assert.NoError(t, json.Unmarshal(r.Spec.ForProvider.Values.Raw, &values))

			if tt.wantNoOverride {
				assert.NotContains(t, values, "image")
				assert.NotContains(t, values, "mcImage")
				return
			}
			assert.Equal(t, tt.wantImage, values["image"].(map[string]interface{})["repository"])
			assert.Equal(t, tt.wantMcImage, values["mcImage"].(map[string]interface{})["repository"])
		})
	}
}

func getMinioComp(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNMinio) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnminio/deploy/01_default.yaml")

	comp := &vshnv1.VSHNMinio{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
