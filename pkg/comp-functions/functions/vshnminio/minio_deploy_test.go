package vshnminio

import (
	"context"
	"testing"

	xhelmbeta1 "github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

func TestMinioDeploy(t *testing.T) {

	iof, comp := getMinioComp(t)

	ctx := context.TODO()

	rootUser := "minio"
	rootPassword := "minio123"
	minioHost := "http://10.0.0.1:9000"

	assert.Equal(t, runtime.NewNormal(), DeployMinio(ctx, iof))

	ns := &corev1.Namespace{}
	assert.NoError(t, iof.Observed.GetFromObject(ctx, ns, comp.Name+"-ns"))

	r := &xhelmbeta1.Release{}
	assert.NoError(t, iof.Observed.Get(ctx, r, comp.Name+"-release"))

	service := &corev1.Service{}
	assert.NoError(t, iof.Observed.GetFromObject(ctx, service, comp.Name+"-service-observer"))

	cd := iof.Desired.GetCompositeConnectionDetails(ctx)
	assert.Equal(t, rootUser, cd[0].Value)
	assert.Equal(t, rootPassword, cd[1].Value)
	assert.Equal(t, minioHost, cd[2].Value)

	sm := &promv1.ServiceMonitor{}
	assert.NoError(t, iof.Desired.GetFromObject(ctx, sm, comp.Name+"-service-monitor"))
	assert.Equal(t, "/minio/v2/metrics/node", sm.Spec.Endpoints[0].Path)
	assert.Equal(t, "/minio/v2/metrics/cluster", sm.Spec.Endpoints[1].Path)
}

func getMinioComp(t *testing.T) (*runtime.Runtime, *vshnv1.VSHNMinio) {
	iof := commontest.LoadRuntimeFromFile(t, "vshnminio/deploy/01_default.yaml")

	comp := &vshnv1.VSHNMinio{}
	err := iof.Observed.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	return iof, comp
}
