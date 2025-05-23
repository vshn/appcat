package vshnredis

/*
import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	// crossfnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xhelm "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestHelmValueUpdate(t *testing.T) {

	svc, _ := getRedisReleaseComp(t, "01_default")
	ctx := context.TODO()

	assert.Nil(t, ManageRelease(ctx, &vshnv1.VSHNRedis{}, svc))

	release := &xhelm.Release{}
	// IMPORTANT: this resource name must not change! Or crossplane will delete the release.
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, "release"))

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

	assert.Nil(t, ManageRelease(ctx, &vshnv1.VSHNRedis{}, iof))

	release := &xhelm.Release{}
	// IMPORTANT: this resource name must not change! Or crossplane will delete the release.
	assert.NoError(t, iof.GetDesiredComposedResourceByName(release, "release"))

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

func getRedisReleaseComp(t *testing.T, file string) (*runtime.ServiceRuntime, *vshnv1.VSHNRedis) {
	svc := commontest.LoadRuntimeFromFile(t, fmt.Sprintf("vshnredis/release/%s.yaml", file))

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}

func TestAllowVersionUpgrade(t *testing.T) {

	tcs := map[string]struct {
		CompositeVersion string
		ObservedVersion  string
		Desired          string
		Fail             bool
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
			Fail:             true,
		},
		"allInvalid": {
			CompositeVersion: "randomTag",
			ObservedVersion:  "weirdTag",
			Desired:          "weirdTag",
			Fail:             true,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {

			ctx := context.TODO()
			iof := loadRuntimeFromTemplate(t, "vshnredis/release/03_version_update.yaml.tmpl", tc)

			res := ManageRelease(ctx, &vshnv1.VSHNRedis{}, iof)
			if tc.Fail {
				return
			}
			assert.Nil(t, res)

			release := &xhelm.Release{}
			assert.NoError(t, iof.GetDesiredComposedResourceByName(release, "release"))

			valueMap := map[string]any{}
			assert.NoError(t, yaml.Unmarshal(release.Spec.ForProvider.Values.Raw, &valueMap))
			require.NotEmpty(t, valueMap["master"])
			imageMap := valueMap["image"].(map[string]any)
			assert.Equal(t, tc.Desired, imageMap["tag"])
		})
	}
}

func loadRuntimeFromTemplate(t assert.TestingT, file string, data any) *runtime.ServiceRuntime {
	p, _ := filepath.Abs(".")
	before, _, _ := strings.Cut(p, "pkg")
	f, err := os.Open(before + "test/functions/" + file)
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

	req, err := commontest.BytesToRequest(buf.Bytes())
	assert.NoError(t, err)

	config := &corev1.ConfigMap{}
	assert.NoError(t, request.GetInput(req, config))

	svc, err := runtime.NewServiceRuntime(logr.Discard(), *config, req)
	assert.NoError(t, err)

	return svc
}
*/
