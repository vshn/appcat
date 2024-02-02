package common

import (
	"testing"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddCredentialsSecret(t *testing.T) {
	comp := &vshnv1.VSHNRedis{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mytest",
		},
	}

	svc := commontest.LoadRuntimeFromFile(t, "empty.yaml")

	res, err := AddCredentialsSecret(comp, svc, []string{"mytest", "mypw"})
	assert.NoError(t, err)
	assert.Equal(t, "mytest-credentials-secret", res)

	secret := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(secret, res))

	assert.Len(t, secret.StringData, 2)
	assert.NotEmpty(t, secret.StringData["mytest"])
	assert.NotEmpty(t, secret.StringData["mypw"])

	obj := &xkube.Object{}

	assert.NoError(t, svc.GetDesiredComposedResourceByName(obj, res))
	assert.NotEmpty(t, obj.Spec.ConnectionDetails)
	assert.Len(t, obj.Spec.ConnectionDetails, 2)

}
