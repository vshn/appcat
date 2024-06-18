package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	corev1 "k8s.io/api/core/v1"
)

func TestAddInitialNamespaceQuotas(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/quotas/01_default.yaml")
	ctx := context.TODO()

	res := addInitialNamespaceQuotas(ctx, svc, "namespace")
	assert.Nil(t, res)

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace"))
	assert.NotEmpty(t, ns.GetAnnotations())
}
