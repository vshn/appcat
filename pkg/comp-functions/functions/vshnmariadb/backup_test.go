package vshnmariadb

import (
	"context"
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_AddBackupMariadb(t *testing.T) {
	svc, comp := getMariadbComp(t)

	assert.Nil(t, AddBackupMariadb(context.TODO(), svc))

	cm := &corev1.ConfigMap{}
	assert.NoError(t, svc.GetDesiredKubeObject(cm, comp.GetName()+"-backup-script"))

	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))

	assert.NotNil(t, release.Spec.ForProvider.Values)

	values := map[string]interface{}{}

	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))

	_, _, err := unstructured.NestedMap(values, "persistence", "annotations")
	assert.NoError(t, err)

	_, _, err = unstructured.NestedStringMap(values, "podAnnotations")
	assert.NoError(t, err)

	_, _, err = unstructured.NestedFieldNoCopy(values, "extraVolumes")
	assert.NoError(t, err)

	_, _, err = unstructured.NestedFieldNoCopy(values, "extraVolumeMounts")
	assert.NoError(t, err)

}
