package vshnredis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func TestRedisDeploy(t *testing.T) {
	svc, comp := getRedisTestComp(t)

	ctx := context.TODO()

	redisUser := "default"
	redisPassword := "redis123"
	redisPort := "6379"
	redisHost := "redis-headless.vshn-redis-redis-gc9x4.svc.cluster.local"
	redisUrl := "rediss://default:redis123@redis-headless.vshn-redis-redis-gc9x4.svc.cluster.local:6379"

	assert.Nil(t, DeployRedis(ctx, comp, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace-conditions"))

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(roleBinding, "namespace-permissions"))

	r := &xhelmbeta1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(r, "release"))

	cd := svc.GetConnectionDetails()
	assert.Equal(t, redisHost, string(cd[redisHostConnectionDetailsField]))
	assert.Equal(t, redisPort, string(cd[redisPortConnectionDetailsField]))
	assert.Equal(t, redisUser, string(cd[redisUsernameConnectionDetailsField]))
	assert.Equal(t, redisPassword, string(cd[redisPasswordConnectionDetailsField]))
	assert.Equal(t, redisUrl, string(cd[redisURLConnectionDetailsField]))
	assert.Nil(t, cd[sentinelHostsConnectionDetailsField])
}

func TestRedisDeployHA(t *testing.T) {
	svc, comp := getRedisTestComp(t)

	comp.Spec.Parameters.Instances = 3

	ctx := context.TODO()

	redisUser := "default"
	redisPassword := "redis123"
	redisPort := "6379"
	redisHost := "redis-master.vshn-redis-redis-gc9x4.svc.cluster.local"
	redisUrl := "rediss://default:redis123@redis-master.vshn-redis-redis-gc9x4.svc.cluster.local:6379"
	sentinelHosts := "redis-node-0.redis-headless.vshn-redis-redis-gc9x4.svc.cluster.local,redis-node-1.redis-headless.vshn-redis-redis-gc9x4.svc.cluster.local,redis-node-2.redis-headless.vshn-redis-redis-gc9x4.svc.cluster.local"

	assert.Nil(t, DeployRedis(ctx, comp, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace-conditions"))

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(roleBinding, "namespace-permissions"))

	r := &xhelmbeta1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(r, "release"))

	cd := svc.GetConnectionDetails()
	assert.Equal(t, redisHost, string(cd[redisHostConnectionDetailsField]))
	assert.Equal(t, redisPort, string(cd[redisPortConnectionDetailsField]))
	assert.Equal(t, redisUser, string(cd[redisUsernameConnectionDetailsField]))
	assert.Equal(t, redisPassword, string(cd[redisPasswordConnectionDetailsField]))
	assert.Equal(t, redisUrl, string(cd[redisURLConnectionDetailsField]))
	assert.Equal(t, sentinelHosts, string(cd[sentinelHostsConnectionDetailsField]))
}

func getRedisTestComp(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNRedis) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnredis/deploy/01_default.yaml")

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
