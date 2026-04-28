package vshnpostgrescnpg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	tcpGatewayDefaultPath  = "vshn-postgres/tcpgateway/01_default.yaml"
	tcpGatewayWithPortPath = "vshn-postgres/tcpgateway/02_with_port.yaml"
)

func TestConfigureTCPGateway(t *testing.T) {

	t.Run("ServiceTypeClusterIP_NoOp", func(t *testing.T) {
		svc, comp := getSvcCompCnpg(t)
		// default fixture has no serviceType set (defaults to ClusterIP)

		result := ConfigureTCPGateway(context.TODO(), comp, svc)
		assert.Nil(t, result)

		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		assert.ErrorIs(t, svc.GetDesiredKubeObject(xls, comp.GetName()+"-xls"), runtime.ErrNotFound)
	})

	t.Run("TCPGateway_ExternalAccessDisabled_Warning", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, tcpGatewayDefaultPath)
		comp := &vshnv1.VSHNPostgreSQL{}
		require.NoError(t, svc.GetObservedComposite(comp))

		// Disable external access
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "false"

		result := ConfigureTCPGateway(context.TODO(), comp, svc)
		require.NotNil(t, result)
		assert.Contains(t, result.Message, "not enabled")
	})

	t.Run("TCPGateway_GatewayConfigMissing_Warning", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, tcpGatewayDefaultPath)
		comp := &vshnv1.VSHNPostgreSQL{}
		require.NoError(t, svc.GetObservedComposite(comp))

		delete(svc.Config.Data, "tcpGatewayNamespace")
		delete(svc.Config.Data, "tcpGateways")

		result := ConfigureTCPGateway(context.TODO(), comp, svc)
		require.NotNil(t, result)
		assert.Contains(t, result.Message, "is not configured")
	})

	t.Run("TCPGateway_CreatesResources", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, tcpGatewayDefaultPath)
		comp := &vshnv1.VSHNPostgreSQL{}
		require.NoError(t, svc.GetObservedComposite(comp))

		result := ConfigureTCPGateway(context.TODO(), comp, svc)
		assert.Nil(t, result)

		// XListenerSet created
		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		assert.NoError(t, svc.GetDesiredKubeObject(xls, comp.GetName()+"-xls"))

		// TCPRoute created
		tcpRoute := &unstructured.Unstructured{}
		tcpRoute.SetAPIVersion("gateway.networking.k8s.io/v1alpha2")
		tcpRoute.SetKind("TCPRoute")
		assert.NoError(t, svc.GetDesiredKubeObject(tcpRoute, comp.GetName()+"-tcproute"))
	})

	t.Run("TCPGateway_PortAllocated_ConnectionDetails", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, tcpGatewayWithPortPath)
		comp := &vshnv1.VSHNPostgreSQL{}
		require.NoError(t, svc.GetObservedComposite(comp))

		result := ConfigureTCPGateway(context.TODO(), comp, svc)
		assert.Nil(t, result)

		cd := svc.GetConnectionDetails()
		assert.Equal(t, "pg.example.com", string(cd["POSTGRESQL_GATEWAY_HOST"]))
		assert.Equal(t, "15432", string(cd["POSTGRESQL_GATEWAY_PORT"]))
	})
}
