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
		assert.ErrorIs(t, svc.GetDesiredKubeObject(xls, comp.GetName()+"-ssh-xls"), runtime.ErrNotFound)
	})

	t.Run("SSHEnabled_GatewayConfigMissing_WarningResult", func(t *testing.T) {
		svc, comp := bootstrapSSHTestFromFixture(t, "vshnforgejo/02_ssh.yaml")

		// Clear gateway config
		delete(svc.Config.Data, "tcpGatewayNamespace")
		delete(svc.Config.Data, "tcpGateways")

		result := ConfigureSSHAccess(context.TODO(), comp, svc)
		require.NotNil(t, result)
		assert.Contains(t, result.Message, "is not configured")
	})

	t.Run("AllocatedPortPreserved_HelmAndConnectionDetails", func(t *testing.T) {
		svc, comp := bootstrapSSHTestFromFixture(t, "vshnforgejo/02_ssh_with_port.yaml")

		result := ConfigureSSHAccess(context.TODO(), comp, svc)
		assert.Nil(t, result)

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

	t.Run("AllocatedGatewayPreserved_ConnectionDetails", func(t *testing.T) {
		// Observed XListenerSet has gateway tcp-gateway-2 (different from config's tcp-gateway)
		svc, comp := bootstrapSSHTestFromFixture(t, "vshnforgejo/02_ssh_with_port_sharded.yaml")

		result := ConfigureSSHAccess(context.TODO(), comp, svc)
		assert.Nil(t, result)

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
