package probes

import (
	"context"
)

var _ Prober = FailingProbe{}

// FailingProbe is a prober that will always fail.
type FailingProbe struct {
	Service           string
	Name              string
	ClaimNamespace    string
	InstanceNamespace string
	Error             error
}

// Close closes open connections.
func (p FailingProbe) Close() error {
	return nil
}

// GetInfo returns the prober infos
func (p FailingProbe) GetInfo() ProbeInfo {
	return ProbeInfo{
		Service:           p.Service,
		Name:              p.Name,
		ClaimNamespace:    p.ClaimNamespace,
		InstanceNamespace: p.InstanceNamespace,
	}
}

// Probe Will always return error, as this is a failing probe.
func (p FailingProbe) Probe(ctx context.Context) error {
	return p.Error
}

// NewFailingProbe creates a prober that will fail.
// Can be used if the controller can't access valid credentials.
func NewFailingProbe(service, name, claimNamespace, instanceNamespace string, err error) (*FailingProbe, error) {
	return &FailingProbe{
		Service:           service,
		Name:              name,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
		Error:             err,
	}, nil
}
