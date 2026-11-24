package vshnredis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewValuesKubectlImage(t *testing.T) {
	tests := []struct {
		name         string
		kubectlImage string
		prefix       string
		want         map[string]any
	}{
		{
			name:         "GivenKubectlImage_ThenChartUsesIt",
			kubectlImage: "quay.io/appuio/oc:v4.22",
			want: map[string]any{
				"registry":   "quay.io",
				"repository": "appuio/oc",
				"tag":        "v4.22",
			},
		},
		{
			name:         "GivenKubectlImageAndPrefix_ThenImageWins",
			kubectlImage: "quay.io/appuio/oc:v4.22",
			prefix:       "vshn",
			want: map[string]any{
				"registry":   "quay.io",
				"repository": "appuio/oc",
				"tag":        "v4.22",
			},
		},
		{
			name:   "GivenPrefixOnly_ThenNoImageIsSet",
			prefix: "vshn",
		},
		{
			name: "GivenNothing_ThenNoImageIsSet",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, comp := getRedisTestComp(t)
			if tc.kubectlImage != "" {
				svc.Config.Data["kubectl_image"] = tc.kubectlImage
			}
			if tc.prefix != "" {
				svc.Config.Data["imageRepositoryPrefix"] = tc.prefix
			}

			values, err := newValues(context.TODO(), svc, comp, "mysecret")
			require.NoError(t, err)

			kubectl, ok := values["kubectl"].(map[string]any)
			require.True(t, ok, "values must contain kubectl")

			image, ok := kubectl["image"].(map[string]any)
			if tc.want == nil {
				assert.False(t, ok, "values must not contain kubectl.image")
				return
			}

			require.True(t, ok, "values must contain kubectl.image")
			assert.Equal(t, tc.want, image)
		})
	}
}

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		name           string
		ref            string
		wantRegistry   string
		wantRepository string
		wantTag        string
	}{
		{
			name:           "GivenRegistryWithTag_ThenSplitAll",
			ref:            "quay.io/appuio/oc:v4.22",
			wantRegistry:   "quay.io",
			wantRepository: "appuio/oc",
			wantTag:        "v4.22",
		},
		{
			name:           "GivenNoRegistry_ThenEmptyRegistry",
			ref:            "appuio/oc:v4.22",
			wantRepository: "appuio/oc",
			wantTag:        "v4.22",
		},
		{
			name:           "GivenNoTag_ThenEmptyTag",
			ref:            "quay.io/appuio/oc",
			wantRegistry:   "quay.io",
			wantRepository: "appuio/oc",
		},
		{
			name:           "GivenRegistryWithPort_ThenKeepPort",
			ref:            "registry.example.com:5000/appuio/oc:v4.22",
			wantRegistry:   "registry.example.com:5000",
			wantRepository: "appuio/oc",
			wantTag:        "v4.22",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, repository, tag := parseImageRef(tc.ref)
			assert.Equal(t, tc.wantRegistry, registry)
			assert.Equal(t, tc.wantRepository, repository)
			assert.Equal(t, tc.wantTag, tag)
		})
	}
}

func getRedisTestComp(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNRedis) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnredis/deploy/01_default.yaml")

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
