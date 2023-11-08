package vshnminio

import (
	"context"
	"testing"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

func TestMinioDeploy(t *testing.T) {

	svc, comp := getMinioComp(t)

	ctx := context.TODO()

	rootUser := "minio"
	rootPassword := "minio123"
	minioHost := "http://10.0.0.1:9000"

	assert.Nil(t, DeployMinio(ctx, svc))

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
}

func getMinioComp(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNMinio) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnminio/deploy/01_default.yaml")

	comp := &vshnv1.VSHNMinio{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
