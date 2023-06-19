package vshnredis

import (
	"context"
	"testing"

	xhelm "github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	appcatv1 "github.com/vshn/appcat/apis/v1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

func TestAddBackupObjectCreation(t *testing.T) {
	iof, comp := getRedisBackupComp(t)

	ctx := context.TODO()

	assert.Equal(t, runtime.NewNormal(), AddBackup(ctx, iof))

	bucket := &appcatv1.XObjectBucket{}
	assert.NoError(t, iof.Desired.Get(ctx, bucket, comp.Name+"-backup"))

	observer := &xkube.Object{}
	assert.NoError(t, iof.Desired.Get(ctx, observer, comp.Name+"-backup-credential-observer"))

	repoPW := &corev1.Secret{}
	assert.NoError(t, iof.Desired.Get(ctx, repoPW, comp.Name+"-k8up-repo-pw"))

	schedule := &k8upv1.Schedule{}
	assert.NoError(t, iof.Desired.Get(ctx, schedule, comp.Name+"-backup-credential-observer"))

	cm := &corev1.ConfigMap{}
	assert.NoError(t, iof.Desired.Get(ctx, cm, comp.Name+"-backup-cm"))

}

func TestHelmValueUpdate(t *testing.T) {

	iof, _ := getRedisBackupComp(t)
	ctx := context.TODO()

	assert.Equal(t, runtime.NewNormal(), AddBackup(ctx, iof))

	release := &xhelm.Release{}
	// IMPORTANT: this resource name must not change! Or crossplane will delete the release.
	assert.NoError(t, iof.Desired.Get(ctx, release, "release"))

	valueMap := map[string]any{}

	assert.NoError(t, yaml.Unmarshal(release.Spec.ForProvider.Values.Raw, &valueMap))

	assert.NotEmpty(t, valueMap["master"])

	persistenceMap := valueMap["master"].(map[string]any)["persistence"].(map[string]any)

	assert.NotEmpty(t, persistenceMap["annotations"])

	masterMap := valueMap["master"].(map[string]any)

	assert.NotEmpty(t, masterMap["podAnnotations"])
	assert.NotEmpty(t, masterMap["extraVolumes"])
	assert.NotEmpty(t, masterMap["extraVolumeMounts"])

}

func getRedisBackupComp(t *testing.T) (*runtime.Runtime, *vshnv1.VSHNRedis) {
	iof := loadRuntimeFromFile(t, "backup/01_default.yaml")

	comp := &vshnv1.VSHNRedis{}
	err := iof.Desired.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	return iof, comp
}
