package vshnredis

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	netv1 "k8s.io/api/networking/v1"
)

func TestRedisLoadBalancer(t *testing.T) {

	t.Run("ServiceTypeClusterIP_NoLoadBalancer", func(t *testing.T) {
		svc, comp := getRedisTestComp(t)

		require.Nil(t, DeployRedis(context.TODO(), comp, svc))

		values := releaseValues(t, svc)
		sentinelService, _ := nestedMap(values, "sentinel", "service")
		assert.Nil(t, sentinelService["type"])

		r := &xhelmbeta1.Release{}
		require.NoError(t, svc.GetDesiredComposedResourceByName(r, redisRelease))
		assert.NotContains(t, connectionDetailKeys(r), loadBalancerIPConnectionDetailsField)
	})

	t.Run("ServiceTypeLoadBalancer_ExternalAccessDisabled_NoOp", func(t *testing.T) {
		svc, comp := getRedisTestComp(t)
		comp.Spec.Parameters.Network.ServiceType = "LoadBalancer"
		// externalDatabaseConnectionsEnabled is not set in the default fixture

		require.Nil(t, DeployRedis(context.TODO(), comp, svc))

		values := releaseValues(t, svc)
		sentinelService, _ := nestedMap(values, "sentinel", "service")
		assert.Nil(t, sentinelService["type"])

		r := &xhelmbeta1.Release{}
		require.NoError(t, svc.GetDesiredComposedResourceByName(r, redisRelease))
		assert.NotContains(t, connectionDetailKeys(r), loadBalancerIPConnectionDetailsField)

		np := &netv1.NetworkPolicy{}
		assert.ErrorIs(t, svc.GetDesiredKubeObject(np, comp.GetName()+"-allow-all"), runtime.ErrNotFound)
	})

	t.Run("ServiceTypeLoadBalancer_SetsServiceAndConnectionDetail", func(t *testing.T) {
		svc, comp := getRedisTestComp(t)
		comp.Spec.Parameters.Network.ServiceType = "LoadBalancer"
		comp.Spec.Parameters.Network.IPFilter = []string{"203.0.113.0/24", "198.51.100.7/32"}
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "true"

		require.Nil(t, DeployRedis(context.TODO(), comp, svc))

		values := releaseValues(t, svc)
		sentinelService, ok := nestedMap(values, "sentinel", "service")
		require.True(t, ok, "sentinel.service must be set")
		assert.Equal(t, "LoadBalancer", sentinelService["type"])

		ranges, ok := sentinelService["loadBalancerSourceRanges"].([]any)
		require.True(t, ok, "loadBalancerSourceRanges must be set")
		assert.ElementsMatch(t, []any{"203.0.113.0/24", "198.51.100.7/32"}, ranges)

		r := &xhelmbeta1.Release{}
		require.NoError(t, svc.GetDesiredComposedResourceByName(r, redisRelease))
		assert.Contains(t, connectionDetailKeys(r), loadBalancerIPConnectionDetailsField)
		assert.Equal(t, "redis", lbConnectionDetailServiceName(r))

		// allow-all netpol must exist so external traffic reaches the pods
		np := &netv1.NetworkPolicy{}
		assert.NoError(t, svc.GetDesiredKubeObject(np, comp.GetName()+"-allow-all"))
	})

	t.Run("ServiceTypeLoadBalancer_HA_UsesMasterService", func(t *testing.T) {
		svc, comp := getRedisTestComp(t)
		comp.Spec.Parameters.Instances = 3
		comp.Spec.Parameters.Network.ServiceType = "LoadBalancer"
		comp.Spec.Parameters.Network.IPFilter = []string{"203.0.113.0/24"}
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "true"

		require.Nil(t, DeployRedis(context.TODO(), comp, svc))

		values := releaseValues(t, svc)

		// the aggregate node service must stay ClusterIP in HA
		nodeService, _ := nestedMap(values, "sentinel", "service")
		assert.Nil(t, nodeService["type"])

		// the master service (isMaster selector, follows failover) is the LB
		masterService, ok := nestedMap(values, "sentinel", "masterService")
		require.True(t, ok)
		assert.Equal(t, "LoadBalancer", masterService["type"])
		// existing enabled key must be preserved by the merge
		assert.Equal(t, true, masterService["enabled"])

		r := &xhelmbeta1.Release{}
		require.NoError(t, svc.GetDesiredComposedResourceByName(r, redisRelease))
		assert.Contains(t, connectionDetailKeys(r), loadBalancerIPConnectionDetailsField)
		assert.Equal(t, "redis-master", lbConnectionDetailServiceName(r))
	})

	t.Run("ServiceTypeClusterIP_NoAllowAllNetpol", func(t *testing.T) {
		svc, comp := getRedisTestComp(t)

		require.Nil(t, DeployRedis(context.TODO(), comp, svc))

		np := &netv1.NetworkPolicy{}
		assert.ErrorIs(t, svc.GetDesiredKubeObject(np, comp.GetName()+"-allow-all"), runtime.ErrNotFound)
	})
}

func releaseValues(t *testing.T, svc *runtime.ServiceRuntime) map[string]any {
	t.Helper()
	r := &xhelmbeta1.Release{}
	require.NoError(t, svc.GetDesiredComposedResourceByName(r, redisRelease))
	values := map[string]any{}
	require.NoError(t, json.Unmarshal(r.Spec.ForProvider.Values.Raw, &values))
	return values
}

func nestedMap(values map[string]any, keys ...string) (map[string]any, bool) {
	current := values
	for _, k := range keys {
		next, ok := current[k].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func connectionDetailKeys(r *xhelmbeta1.Release) []string {
	keys := make([]string, 0, len(r.Spec.ConnectionDetails))
	for _, cd := range r.Spec.ConnectionDetails {
		keys = append(keys, cd.ToConnectionSecretKey)
	}
	return keys
}

func lbConnectionDetailServiceName(r *xhelmbeta1.Release) string {
	for _, cd := range r.Spec.ConnectionDetails {
		if cd.ToConnectionSecretKey == loadBalancerIPConnectionDetailsField {
			return cd.Name
		}
	}
	return ""
}
