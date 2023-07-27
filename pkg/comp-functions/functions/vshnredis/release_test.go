package vshnredis

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	xhelm "github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	"sigs.k8s.io/yaml"
)

func TestHelmValueUpdate(t *testing.T) {

	iof, _ := getRedisReleaseComp(t, "01_default")
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

	assert.Equal(t, "7.0.11", imageMap["tag"])

	masterMap := valueMap["master"].(map[string]any)

	assert.NotEmpty(t, masterMap["podAnnotations"])
	assert.NotEmpty(t, masterMap["extraVolumes"])
	assert.NotEmpty(t, masterMap["extraVolumeMounts"])

	assert.Equal(t, "updated", valueMap["commonConfiguration"])
}

func TestHelmValueUpdate_noObservered(t *testing.T) {

	iof, _ := getRedisReleaseComp(t, "02_no_observed")
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

	assert.Equal(t, "updated", valueMap["commonConfiguration"])
}

func getRedisReleaseComp(t *testing.T, file string) (*runtime.Runtime, *vshnv1.VSHNRedis) {
	iof := commontest.LoadRuntimeFromFile(t, fmt.Sprintf("vshnredis/release/%s.yaml", file))

	comp := &vshnv1.VSHNRedis{}
	err := iof.Desired.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	return iof, comp
}

func TestAllowVersionUpgrade(t *testing.T) {

	tcs := map[string]struct {
		CompositeVersion string
		ObservedVersion  string
		Desired          string
	}{
		"newMinor": {
			CompositeVersion: "6.2",
			ObservedVersion:  "6.0.2",
			Desired:          "6.2",
		},
		"newMajor": {
			CompositeVersion: "7.0",
			ObservedVersion:  "6.2.2",
			Desired:          "7.0",
		},
		"sameMinior": {
			CompositeVersion: "6.2",
			ObservedVersion:  "6.2.2",
			Desired:          "6.2.2",
		},
		"sameVersion": {
			CompositeVersion: "6.2",
			ObservedVersion:  "6.2",
			Desired:          "6.2",
		},
		"noObserved": {
			CompositeVersion: "6.2",
			ObservedVersion:  "",
			Desired:          "6.2",
		},
		"downgrade": {
			CompositeVersion: "6.2",
			ObservedVersion:  "7.0.2",
			Desired:          "7.0.2",
		},
		"invalidObserved": {
			CompositeVersion: "6.2",
			ObservedVersion:  "randomTag",
			Desired:          "6.2",
		},
		"invalidDesired": {
			CompositeVersion: "randomTag",
			ObservedVersion:  "7.0.3",
			Desired:          "7.0.3",
		},
		"allInvalid": {
			CompositeVersion: "randomTag",
			ObservedVersion:  "weirdTag",
			Desired:          "weirdTag",
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {

			ctx := context.TODO()
			iof := loadRuntimeFromTemplate(t, fmt.Sprintf("vshnredis/release/03_version_update.yaml.tmpl"), tc)
			assert.Equal(t, runtime.NewNormal(), ManageRelease(ctx, iof))
			release := &xhelm.Release{}
			assert.NoError(t, iof.Desired.Get(ctx, release, "release"))

			valueMap := map[string]any{}
			assert.NoError(t, yaml.Unmarshal(release.Spec.ForProvider.Values.Raw, &valueMap))
			require.NotEmpty(t, valueMap["master"])
			imageMap := valueMap["image"].(map[string]any)
			assert.Equal(t, tc.Desired, imageMap["tag"])
		})
	}
}

func loadRuntimeFromTemplate(t assert.TestingT, file string, data any) *runtime.Runtime {
	p, _ := filepath.Abs(".")
	before, _, _ := strings.Cut(p, "pkg")
	f, err := os.Open(before + "test/transforms/" + file)
	assert.NoError(t, err)
	b1, err := os.ReadFile(f.Name())
	if err != nil {
		assert.FailNow(t, "can't get example")
	}

	tmpl, err := template.New("test").Parse(string(b1))
	if err != nil {
		assert.FailNow(t, "can't load tempalte")
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		assert.FailNow(t, "can't execute tempalte")
	}

	funcIO, err := runtime.NewRuntime(context.Background(), buf.Bytes())
	assert.NoError(t, err)

	return funcIO
}
