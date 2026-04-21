package vshnforgejo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSSH(t *testing.T) {

	t.Run("SSHDisabled_NoResourcesCreated", func(t *testing.T) {
		// Use default fixture which has SSH disabled
		svc, comp, _ := bootstrapTest(t)

		result := ConfigureSSHAccess(context.TODO(), comp, svc)
		assert.Nil(t, result)

		// Verify no SSH-related resources were created
		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		assert.ErrorIs(t, svc.GetDesiredKubeObject(xls, comp.GetName()+"-ssh"), runtime.ErrNotFound)
	})

	t.Run("SSHEnabled_AllResourcesCreated", func(t *testing.T) {
		svc, comp := bootstrapSSHTestFromFixture(t, "vshnforgejo/02_ssh.yaml")

		result := ConfigureSSHAccess(context.TODO(), comp, svc)
		assert.Nil(t, result)

		resourceBaseName := comp.GetName() + "-ssh"
		instanceNs := comp.GetInstanceNamespace()

		// Verify XListenerSet
		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		require.NoError(t, svc.GetDesiredKubeObject(xls, resourceBaseName))
		assert.Equal(t, resourceBaseName, xls.GetName())
		assert.Equal(t, "gateway-system", xls.GetNamespace())
		assert.Equal(t, "true", xls.GetLabels()["appcat.vshn.io/sshgateway"])

		parentName, _, _ := unstructured.NestedString(xls.Object, "spec", "parentRef", "name")
		parentNs, _, _ := unstructured.NestedString(xls.Object, "spec", "parentRef", "namespace")
		assert.Equal(t, "tcp-gateway", parentName)
		assert.Equal(t, "gateway-system", parentNs)

		listeners, found, _ := unstructured.NestedSlice(xls.Object, "spec", "listeners")
		require.True(t, found)
		require.Len(t, listeners, 1)
		l0 := listeners[0].(map[string]any)
		assert.Equal(t, "ssh", l0["name"])
		assert.Equal(t, "TCP", l0["protocol"])
		assert.Equal(t, int64(0), l0["port"]) // 0 on first create, webhook assigns

		// Verify allowedRoutes scoped to instance namespace
		fromMode, _, _ := unstructured.NestedString(l0, "allowedRoutes", "namespaces", "from")
		assert.Equal(t, "Selector", fromMode)
		selectorLabel, _, _ := unstructured.NestedString(l0, "allowedRoutes", "namespaces", "selector", "matchLabels", "kubernetes.io/metadata.name")
		assert.Equal(t, instanceNs, selectorLabel)

		// Verify TCPRoute
		tcpRoute := &unstructured.Unstructured{}
		tcpRoute.SetAPIVersion("gateway.networking.k8s.io/v1alpha2")
		tcpRoute.SetKind("TCPRoute")
		require.NoError(t, svc.GetDesiredKubeObject(tcpRoute, resourceBaseName+"-tcproute"))
		assert.Equal(t, resourceBaseName, tcpRoute.GetName())
		assert.Equal(t, instanceNs, tcpRoute.GetNamespace())

		parentRefs, _, _ := unstructured.NestedSlice(tcpRoute.Object, "spec", "parentRefs")
		require.Len(t, parentRefs, 1)
		pRef := parentRefs[0].(map[string]any)
		assert.Equal(t, "XListenerSet", pRef["kind"])
		assert.Equal(t, resourceBaseName, pRef["name"])
		assert.Equal(t, "ssh", pRef["sectionName"])

		rules, _, _ := unstructured.NestedSlice(tcpRoute.Object, "spec", "rules")
		require.Len(t, rules, 1)
		backendRefs := rules[0].(map[string]any)["backendRefs"].([]any)
		require.Len(t, backendRefs, 1)
		backend := backendRefs[0].(map[string]any)
		assert.Equal(t, "Service", backend["kind"])
		assert.Equal(t, resourceBaseName, backend["name"]) // <comp-name>-ssh
		assert.Equal(t, instanceNs, backend["namespace"])
		assert.Equal(t, int64(22), backend["port"])

		// Verify Helm release does not have SSH enabled yet
		release := &xhelmv1.Release{}
		require.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()))
		values := getReleaseValues(t, *release)
		serverConfig := values["gitea"].(map[string]any)["config"].(map[string]any)["server"].(map[string]any)
		assert.Equal(t, true, serverConfig["DISABLE_SSH"], "SSH should stay disabled until a port is allocated")
		assert.Nil(t, serverConfig["START_SSH_SERVER"], "START_SSH_SERVER should not be set until a port is allocated")

		// Verify NetworkPolicy
		netPol := &netv1.NetworkPolicy{}
		require.NoError(t, svc.GetDesiredKubeObject(netPol, resourceBaseName+"-netpol"))
		assert.Equal(t, resourceBaseName, netPol.Name)
		assert.Equal(t, instanceNs, netPol.Namespace)
		assert.Equal(t, "forgejo", netPol.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
		assert.Equal(t, comp.GetName(), netPol.Spec.PodSelector.MatchLabels["app.kubernetes.io/instance"], "NetworkPolicy should target Forgejo pods")
		require.Len(t, netPol.Spec.Ingress, 1)
		require.Len(t, netPol.Spec.Ingress[0].From, 1)
		assert.Equal(t, "gateway-system", netPol.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"])
		require.Len(t, netPol.Spec.Ingress[0].Ports, 1)
		assert.Equal(t, int32(2222), netPol.Spec.Ingress[0].Ports[0].Port.IntVal, "NetworkPolicy should allow traffic on the SSH pod listen port")

	})

	t.Run("SSHEnabled_GatewayConfigMissing_WarningResult", func(t *testing.T) {
		svc, comp := bootstrapSSHTestFromFixture(t, "vshnforgejo/02_ssh.yaml")

		// Clear gateway config
		delete(svc.Config.Data, "sshGatewayNamespace")
		delete(svc.Config.Data, "sshGateways")

		result := ConfigureSSHAccess(context.TODO(), comp, svc)
		require.NotNil(t, result)
		assert.Contains(t, result.Message, "sshGatewayNamespace or sshGateways is not configured")
	})

	t.Run("AllocatedPortPreserved_OnSubsequentReconcile", func(t *testing.T) {
		svc, comp := bootstrapSSHTestFromFixture(t, "vshnforgejo/02_ssh_with_port.yaml")

		result := ConfigureSSHAccess(context.TODO(), comp, svc)
		assert.Nil(t, result)

		// Verify the desired XListenerSet preserves the allocated port
		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		require.NoError(t, svc.GetDesiredKubeObject(xls, comp.GetName()+"-ssh"))
		listeners, _, _ := unstructured.NestedSlice(xls.Object, "spec", "listeners")
		require.Len(t, listeners, 1)
		l0 := listeners[0].(map[string]any)
		assert.Equal(t, int64(10005), l0["port"])

		// Verify Helm release has SSH settings
		release := &xhelmv1.Release{}
		require.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()))
		values := getReleaseValues(t, *release)
		serverConfig := values["gitea"].(map[string]any)["config"].(map[string]any)["server"].(map[string]any)
		assert.Equal(t, false, serverConfig["DISABLE_SSH"])
		assert.Equal(t, true, serverConfig["START_SSH_SERVER"])
		assert.Equal(t, "ssh.example.com", serverConfig["SSH_DOMAIN"])
		// json.Marshal produces float64 for numbers
		assert.Equal(t, float64(10005), serverConfig["SSH_PORT"])

		// Verify connection details are set
		cd := svc.GetConnectionDetails()
		assert.Equal(t, "ssh.example.com", string(cd["FORGEJO_SSH_HOST"]))
		assert.Equal(t, "10005", string(cd["FORGEJO_SSH_PORT"]))
	})

	t.Run("AllocatedGatewayPreserved_OnSubsequentReconcile", func(t *testing.T) {
		// Observed XListenerSet has gateway tcp-gateway-2 (different from config's tcp-gateway)
		svc, comp := bootstrapSSHTestFromFixture(t, "vshnforgejo/02_ssh_with_port_sharded.yaml")

		result := ConfigureSSHAccess(context.TODO(), comp, svc)
		assert.Nil(t, result)

		// Verify the desired XListenerSet preserves the observed gateway
		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		require.NoError(t, svc.GetDesiredKubeObject(xls, comp.GetName()+"-ssh"))

		parentName, _, _ := unstructured.NestedString(xls.Object, "spec", "parentRef", "name")
		parentNs, _, _ := unstructured.NestedString(xls.Object, "spec", "parentRef", "namespace")
		assert.Equal(t, "tcp-gateway-2", parentName, "gateway name should be preserved from observed state")
		assert.Equal(t, "gateway-system", parentNs)

		// Port should still be preserved
		listeners, _, _ := unstructured.NestedSlice(xls.Object, "spec", "listeners")
		require.Len(t, listeners, 1)
		l0 := listeners[0].(map[string]any)
		assert.Equal(t, int64(10005), l0["port"])

		// Verify connection details use the sharded gateway's domain
		cd := svc.GetConnectionDetails()
		assert.Equal(t, "ssh2.example.com", string(cd["FORGEJO_SSH_HOST"]))
		assert.Equal(t, "10005", string(cd["FORGEJO_SSH_PORT"]))
	})
}

// bootstrapSSHTestFromFixture loads the given SSH fixture and runs addForgejo to create
// the Helm release in desired resources (required by enableSSHInRelease).
func bootstrapSSHTestFromFixture(t *testing.T, fixture string) (*runtime.ServiceRuntime, *vshnv1.VSHNForgejo) {
	t.Helper()

	svc := commontest.LoadRuntimeFromFile(t, fixture)

	comp := &vshnv1.VSHNForgejo{}
	require.NoError(t, svc.GetObservedComposite(comp))

	secretName, err := common.AddCredentialsSecret(comp, svc, []string{"password"}, common.DisallowDeletion, common.AddStaticFieldToSecret(map[string]string{
		"username": "forgejo_admin",
	}))
	require.NoError(t, err)

	require.NoError(t, addForgejo(context.TODO(), svc, comp, secretName))

	return svc, comp
}
