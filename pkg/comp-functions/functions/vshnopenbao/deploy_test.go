package vshnopenbao

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func TestDeployOpenBao(t *testing.T) {
	svc, comp := getOpenBaoTestComp(t)

	ctx := context.TODO()

	assert.Nil(t, DeployOpenBao(ctx, comp, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetObservedKubeObject(ns, comp.Name+"-ns"))

	r := &xhelmbeta1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(r, comp.Name+"-release"))

	var values map[string]interface{}
	assert.NoError(t, json.Unmarshal(r.Spec.ForProvider.Values.Raw, &values))
}
