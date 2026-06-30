package vshnmariadb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func TestMariaDBConfigureTCPGateway(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnmariadb/tcpgateway/02_with_port.yaml")
	comp := &vshnv1.VSHNMariaDB{}
	require.NoError(t, svc.GetObservedComposite(comp))

	result := ConfigureTCPGateway(context.TODO(), comp, svc)
	assert.Nil(t, result)

	// gateway connection details published from observed allocated port + configured domain
	cd := svc.GetConnectionDetails()
	assert.Equal(t, "mariadb.example.com", string(cd["MARIADB_GATEWAY_HOST"]))
	assert.Equal(t, "13306", string(cd["MARIADB_GATEWAY_PORT"]))
}
