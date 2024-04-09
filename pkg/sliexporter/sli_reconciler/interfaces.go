package slireconciler

import (
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service interface {
	client.Object
	resource.Managed
}

type ProbeManager interface {
	StartProbe(p probes.Prober)
	StopProbe(p probes.ProbeInfo)
}
