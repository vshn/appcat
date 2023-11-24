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
		result := AddLoadBalancerIPToConnectionDetails(ctx, svc)

		desired := svc.GetAllDesired()

		if len(desired) != 2 {
			t.Fatal("Expected 2 resources in desired, got", len(desired))
		}

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
		result := AddLoadBalancerIPToConnectionDetails(ctx, svc)

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
		result := AddLoadBalancerIPToConnectionDetails(ctx, svc)

		// Then
		assert.Nil(t, result)
	})
}

// this test normally should return Warning because it's copy of TestLoadBalancerParameterSet()
// but due to disabled LoadBalancer in config it returns Normal
func TestLoadBalancerNotEnabled(t *testing.T) {
	ctx := context.Background()

	t.Run("Verify composition", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/loadbalancer/01-LoadBalancerSet.yaml")
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "false"

		// When
		result := AddLoadBalancerIPToConnectionDetails(ctx, svc)

		// Then
		assert.Nil(t, result)
	})
}
