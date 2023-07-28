package vshnpostgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
)

func TestNothingToDo(t *testing.T) {
	ctx := context.Background()
	expectResult := runtime.NewNormal()

	t.Run("NothingToDo", func(t *testing.T) {

		//Given
		io := commontest.LoadRuntimeFromFile(t, "vshn-postgres/maintenance/01-GivenSchedule.yaml")

		// When
		result := AddLoadBalancerIPToConnectionDetails(ctx, io)

		// Then
		assert.Equal(t, expectResult, result)
	})
}

func TestLoadBalancerParameterSet(t *testing.T) {
	ctx := context.Background()
	// it need another reconciliation to get the service observer object
	expectResult := runtime.NewWarning(ctx, "Cannot yet get service observer object")

	t.Run("Verify composition", func(t *testing.T) {

		//Given
		iof := commontest.LoadRuntimeFromFile(t, "vshn-postgres/loadbalancer/01-LoadBalancerSet.yaml")

		// When
		result := AddLoadBalancerIPToConnectionDetails(ctx, iof)

		// Then
		assert.Equal(t, expectResult, result)
	})
}

func TestLoadBalancerServiceObserverCreated(t *testing.T) {
	ctx := context.Background()
	// file with service observer present, expect NewNormal
	expectResult := runtime.NewNormal()

	t.Run("Verify composition", func(t *testing.T) {

		//Given
		iof := commontest.LoadRuntimeFromFile(t, "vshn-postgres/loadbalancer/02-ServiceObserverPresent.yaml")

		// When
		result := AddLoadBalancerIPToConnectionDetails(ctx, iof)

		// Then
		assert.Equal(t, expectResult, result)
	})
}
