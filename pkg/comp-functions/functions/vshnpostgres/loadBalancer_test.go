package vshnpostgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func TestNothingToDo(t *testing.T) {
	ctx := context.Background()

	t.Run("NothingToDo", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/loadbalancer/01-LoadBalancerSet.yaml")
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "true"

		// When
		result := AddPrimaryService(ctx, svc)

		desired := svc.GetAllDesired()

		assert.Len(t, desired, 2)

		// Then
		assert.NotNil(t, result)
	})
}

func TestLoadBalancerParameterSet(t *testing.T) {
	ctx := context.Background()

	t.Run("Verify composition", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/loadbalancer/01-LoadBalancerSet.yaml")

		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "true"

		// When
		result := AddPrimaryService(ctx, svc)

		// Then
		assert.NotNil(t, result)
	})
}

func TestLoadBalancerServiceObserverCreated(t *testing.T) {
	ctx := context.Background()

	t.Run("Verify composition", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/loadbalancer/02-ServiceObserverPresent.yaml")
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "true"

		// When
		result := AddPrimaryService(ctx, svc)

		// Then
		assert.Nil(t, result)
	})
}

// The primary-service should be deployed even if loadbalancing is disabled.
// However it should not be allowed to be set to type LoadBalancer.
func TestLoadBalancerNotEnabled(t *testing.T) {
	ctx := context.Background()

	t.Run("Verify composition", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/loadbalancer/02-ServiceObserverPresent.yaml")
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "false"

		// When
		result := AddPrimaryService(ctx, svc)

		// Then
		assert.Nil(t, result)
	})
}
