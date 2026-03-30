package sshgateway

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
// Returns an error if all gateways are full.
func (gs *GatewaySharding) SelectGateway(currentRef GatewayKey, newListenerCount int, listenerCounts map[GatewayKey]int) (GatewayKey, bool, error) {
	if slices.Contains(gs.gateways, currentRef) && listenerCounts[currentRef]+newListenerCount <= gs.capacity {
		return currentRef, false, nil
	}

	best := GatewayKey{}
	bestCount := gs.capacity + 1
	for _, gw := range gs.gateways {
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
