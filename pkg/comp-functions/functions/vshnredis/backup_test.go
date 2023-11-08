package vshnredis

import (
	"context"
	"testing"

	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

func TestAddBackupObjectCreation(t *testing.T) {
	svc, comp := getRedisBackupComp(t)

	ctx := context.TODO()

	assert.Nil(t, AddBackup(ctx, svc))

	bucket := &appcatv1.XObjectBucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, comp.Name+"-backup"))

	observer := &xkube.Object{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(observer, comp.Name+"-backup-credential-observer"))

	repoPW := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(repoPW, comp.Name+"-k8up-repo-pw"))

	schedule := &k8upv1.Schedule{}
	assert.NoError(t, svc.GetDesiredKubeObject(schedule, comp.Name+"-backup-credential-observer"))

	cm := &corev1.ConfigMap{}
	assert.NoError(t, svc.GetDesiredKubeObject(cm, comp.Name+"-backup-cm"))

}

func getRedisBackupComp(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNRedis) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnredis/backup/01_default.yaml")

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetDesiredComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
