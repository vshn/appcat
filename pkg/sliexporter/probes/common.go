package probes

import (
	"context"
)

var _ Prober = FailingProbe{}

// FailingProbe is a prober that will always fail.
type FailingProbe struct {
	Service   string
	Name      string
	Namespace string
	Error     error
}

// Close closes open connections.
func (p FailingProbe) Close() error {
	return nil
}

// GetInfo returns the prober infos
func (p FailingProbe) GetInfo() ProbeInfo {
	return ProbeInfo{
		Service:   p.Service,
		Name:      p.Name,
		Namespace: p.Namespace,
	}
}

// Will always return error, as this is a failing probe.
func (p FailingProbe) Probe(ctx context.Context) error {
	return p.Error
}

// NewFailing creates a prober that will fail.
// Can be used if the controller can't access valid credentials.
func NewFailingProbe(service, name, namespace string, err error) (*FailingProbe, error) {
	return &FailingProbe{
		Service:   service,
		Name:      name,
		Namespace: namespace,
		Error:     err,
	}, nil
}
