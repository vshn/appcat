package vshnredis

import (
	"context"
	xhelm "github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	"sigs.k8s.io/yaml"
	"testing"
)

func TestHelmValueUpdate(t *testing.T) {

	iof, _ := getRedisReleaseComp(t)
	ctx := context.TODO()

	assert.Equal(t, runtime.NewNormal(), ManageRelease(ctx, iof))

	release := &xhelm.Release{}
	// IMPORTANT: this resource name must not change! Or crossplane will delete the release.
	assert.NoError(t, iof.Desired.Get(ctx, release, "release"))

	valueMap := map[string]any{}

	assert.NoError(t, yaml.Unmarshal(release.Spec.ForProvider.Values.Raw, &valueMap))

	assert.NotEmpty(t, valueMap["master"])

	persistenceMap := valueMap["master"].(map[string]any)["persistence"].(map[string]any)

	assert.NotEmpty(t, persistenceMap["annotations"])

	imageMap := valueMap["image"].(map[string]any)

	assert.Equal(t, "7.0", imageMap["tag"])

	masterMap := valueMap["master"].(map[string]any)

	assert.NotEmpty(t, masterMap["podAnnotations"])
	assert.NotEmpty(t, masterMap["extraVolumes"])
	assert.NotEmpty(t, masterMap["extraVolumeMounts"])
}

func getRedisReleaseComp(t *testing.T) (*runtime.Runtime, *vshnv1.VSHNRedis) {
	iof := commontest.LoadRuntimeFromFile(t, "vshnredis/release/01_default.yaml")

	comp := &vshnv1.VSHNRedis{}
	err := iof.Desired.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	return iof, comp
}
