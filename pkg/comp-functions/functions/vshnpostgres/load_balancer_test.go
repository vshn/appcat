package vshnpostgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func TestNothingToDoLoadBalancer(t *testing.T) {
	ctx := context.Background()

	t.Run("NothingToDo", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/service_type/01-LoadBalancerSet.yaml")
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "true"

		// When
		result := AddPrimaryService(ctx, &vshnv1.VSHNPostgreSQL{}, svc)

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
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/service_type/01-LoadBalancerSet.yaml")
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "true"

		// When
		result := AddPrimaryService(ctx, &vshnv1.VSHNPostgreSQL{}, svc)

		// Then
		assert.NotNil(t, result)
	})
}

func TestLoadBalancerServiceObserverCreated(t *testing.T) {
	ctx := context.Background()

	t.Run("Verify composition", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/service_type/02-ServiceObserverPresent.yaml")
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "true"

		// When
		result := AddPrimaryService(ctx, &vshnv1.VSHNPostgreSQL{}, svc)

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
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/service_type/02-ServiceObserverPresent.yaml")
		svc.Config.Data["externalDatabaseConnectionsEnabled"] = "false"

		// When
		result := AddPrimaryService(ctx, &vshnv1.VSHNPostgreSQL{}, svc)

		// Then
		assert.Nil(t, result)
	})
}

// Test that empty ServiceType is treated as ClusterIP (the default)
func TestEmptyServiceTypeDefaultsToClusterIP(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty ServiceType should return nil like ClusterIP", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/service_type/03-EmptyServiceType.yaml")

		// When
		result := AddPrimaryService(ctx, &vshnv1.VSHNPostgreSQL{}, svc)

		// Then
		// Should return nil immediately without trying to access LoadBalancer status
		assert.Nil(t, result)
	})
}

// Test that ClusterIP ServiceType
func TestServiceTypeClusterIP(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty ServiceType should return nil like ClusterIP", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/service_type/04-ClusterIPSet.yaml")

		// When
		result := AddPrimaryService(ctx, &vshnv1.VSHNPostgreSQL{}, svc)

		// Then
		// Should return nil immediately without trying to access LoadBalancer status
		assert.Nil(t, result)
	})
}
