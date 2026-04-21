package vshnpostgrescnpg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func Test_getLoadBalancerConnectionDetail(t *testing.T) {
	svc, comp := getSvcCompCnpg(t)
	comp.Spec.Parameters.Network.ServiceType = string(corev1.ServiceTypeLoadBalancer)

	cd := getLoadBalancerConnectionDetail(comp, svc)

	assert.Equal(t, "v1", cd.APIVersion)
	assert.Equal(t, "Service", cd.Kind)
	assert.Equal(t, "primary", cd.Name)
	assert.Equal(t, comp.GetInstanceNamespace(), cd.Namespace)
	assert.Equal(t, "status.loadBalancer.ingress[0].ip", cd.FieldPath)
	assert.Equal(t, "LOADBALANCER_IP", cd.ToConnectionSecretKey)
	assert.True(t, cd.SkipPartOfReleaseCheck)
}
