package vshnnextcloud

import (
	"context"
	_ "embed"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func Test_addNextcloud(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/01_default.yaml")

	ctx := context.TODO()

	assert.Nil(t, DeployNextcloud(ctx, svc))

	pg := &vshnv1.XVSHNPostgreSQL{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+pgInstanceNameSuffix))

	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))

	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))
	extDb := values["externalDatabase"].(map[string]any)
	assert.Equal(t, true, extDb["enabled"])
	intDb := values["internalDatabase"].(map[string]any)
	assert.Equal(t, false, intDb["enabled"])

}

func Test_addReleaseInternalDB(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/02_no_pg.yaml")

	ctx := context.TODO()

	assert.Nil(t, DeployNextcloud(ctx, svc))

	pg := &vshnv1.XVSHNPostgreSQL{}
	assert.Error(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+pgInstanceNameSuffix))

	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))

	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))
	extDb := values["externalDatabase"].(map[string]any)
	assert.Equal(t, map[string]any{}, extDb)
	intDb := values["internalDatabase"].(map[string]any)
	assert.Equal(t, true, intDb["enabled"])
}

//go:embed files/vshn-nextcloud.config.php
var testNextcloudConfig string

func Test_setBackgroundJobMaintenance(t *testing.T) {

	tests := []struct {
		name      string
		timeOfDay string
		want      string
	}{
		{
			name:      "10MinEarlierThan20Expect21",
			timeOfDay: "19:50:01",
			want:      "21",
		},
		{
			name:      "21MinEarlierThan20Expect21",
			timeOfDay: "19:39:01",
			want:      "21",
		},
		{
			name:      "50MinEarlierThan20Expect20",
			timeOfDay: "19:10:01",
			want:      "20",
		},
		{
			name:      "1MinAfterThan0Expect1",
			timeOfDay: "00:01:01",
			want:      "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedNextcloudConfig := setBackgroundJobMaintenance(vshnv1.TimeOfDay(tt.timeOfDay), testNextcloudConfig)
			splitted := strings.Split(updatedNextcloudConfig, "'maintenance_window_start' => ")
			actual := strings.Trim(splitted[1][:2], ",\n")
			assert.Equal(t, tt.want, actual)
		})
	}
}

func getNextcloudComp(t *testing.T, file string) (*runtime.ServiceRuntime, *vshnv1.VSHNNextcloud) {
	svc := commontest.LoadRuntimeFromFile(t, file)

	comp := &vshnv1.VSHNNextcloud{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
