package tcpgateway

import (
	"fmt"
	"slices"
)

// GatewaySharding selects the best Gateway for new XListenerSets based on
// per-gateway listener capacity.
type GatewaySharding struct {
	gateways []GatewayKey
	capacity int
}

// NewGatewaySharding creates a new GatewaySharding instance.
// capacity is the maximum number of listeners per gateway.
func NewGatewaySharding(gateways []GatewayKey, capacity int) *GatewaySharding {
	return &GatewaySharding{
		gateways: gateways,
		capacity: capacity,
	}
}

// SelectGateway determines which gateway should host the new XListenerSet.
// If the current gateway has room, it returns unchanged.
// Otherwise it picks the gateway with the fewest listeners that still has capacity.
// If allowedGateways is non-empty, only those gateways are considered as candidates.
// Returns an error if all candidate gateways are full.
func (gs *GatewaySharding) SelectGateway(currentRef GatewayKey, newListenerCount int, listenerCounts map[GatewayKey]int, allowedGateways []GatewayKey) (GatewayKey, bool, error) {
	candidates := gs.gateways
	if len(allowedGateways) > 0 {
		candidates = gs.filterAllowed(allowedGateways)
	}

	if slices.Contains(candidates, currentRef) && listenerCounts[currentRef]+newListenerCount <= gs.capacity {
		return currentRef, false, nil
	}

	best := GatewayKey{}
	bestCount := gs.capacity + 1
	for _, gw := range candidates {
		count := listenerCounts[gw]
		if count+newListenerCount <= gs.capacity && count < bestCount {
			best = gw
			bestCount = count
		}
	}

	if bestCount > gs.capacity {
		return GatewayKey{}, false, fmt.Errorf("all gateways are full (capacity %d): cannot place %d new listener(s)", gs.capacity, newListenerCount)
	}

	return best, true, nil
}

// filterAllowed returns gateways present in both gs.gateways and allowed.
func (gs *GatewaySharding) filterAllowed(allowed []GatewayKey) []GatewayKey {
	var result []GatewayKey
	for _, gw := range gs.gateways {
		if slices.Contains(allowed, gw) {
			result = append(result, gw)
		}
	}
	return result
}
