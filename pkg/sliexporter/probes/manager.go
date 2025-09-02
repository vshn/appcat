package probes

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	maintenancecontroller "github.com/vshn/appcat/v4/pkg/sliexporter/maintenance_controller"
)

// Manager manages a collection of Probers the check connectivity to AppCat services.
type Manager struct {
	hist prometheus.ObserverVec
	log  logr.Logger

	mu                *sync.Mutex
	probers           map[key]context.CancelFunc
	maintenanceStatus maintenancecontroller.MaintenanceStatus

	newTicker func() (<-chan time.Time, func())
}

// Prober checks the connectivity to an AppCat service
type Prober interface {
	GetInfo() ProbeInfo
	Probe(ctx context.Context) error
	Close() error
}

// ProbeInfo contains meta information on a prober and in turn an AppCat service
type ProbeInfo struct {
	Service       string
	Name          string
	Namespace     string
	Organization  string
	HighAvailable bool
	ServiceLevel  string
}

func NewProbeInfo(serviceKey string, nn types.NamespacedName, o client.Object) ProbeInfo {
	namespace := nn.Namespace
	if namespace == "" {
		namespace = o.GetLabels()["crossplane.io/claim-namespace"]
	}
	return ProbeInfo{
		Service:   serviceKey,
		Name:      o.GetName(),
		Namespace: namespace,
	}
}

// key uniquely identifies a prober
type key string

func getKey(pi ProbeInfo) key {
	return key(fmt.Sprintf("%s; %s", pi.Service, pi.Name))
}

// ErrTimeout is the error thrown when the probe can't reach the endpoint for a longer period of time.
var ErrTimeout = errors.New("probe timed out")

// NewManager returns a new manager.
func NewManager(l logr.Logger, maintenanceStatus maintenancecontroller.MaintenanceStatus) Manager {
	hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "appcat_probes_seconds",
		Help:    "Latency of probes to appact services",
		Buckets: []float64{0.001, 0.002, 0.003, 0.004, 0.005, 0.01, 0.015, 0.02, 0.025, 0.05, 0.1, .5, 1},
	}, []string{"service", "namespace", "name", "reason", "organization", "ha", "sla", "maintenance"})

	return Manager{
		hist:              hist,
		log:               l,
		mu:                &sync.Mutex{},
		probers:           map[key]context.CancelFunc{},
		newTicker:         newTickerChan,
		maintenanceStatus: maintenanceStatus,
	}
}

// Collectors returns all collectors including service-specific ones
func (m Manager) Collectors() []prometheus.Collector {
	collectors := []prometheus.Collector{m.hist}

	collectors = append(collectors, GetRedisCollectors()...)

	return collectors
}

// StartProbe will send a probe once every second using the provided prober.
// If a prober with the same ProbeInfo already runs, it will stop the running prober.
func (m Manager) StartProbe(p Prober) {
	m.mu.Lock()
	defer m.mu.Unlock()

	l := m.log.WithValues("namespace", p.GetInfo().Namespace, "name", p.GetInfo().Name)
	probeKey := getKey(p.GetInfo())
	cancel, ok := m.probers[probeKey]
	if ok {
		l.Info("Cancel Probe")
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.probers[probeKey] = cancel
	l.Info("Start Probe")
	go m.runProbe(ctx, p)
}

// StopProbe will stop the prober with the provided ProbeInfo.
// Is a Noop if none is running.
func (m Manager) StopProbe(pi ProbeInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	probeKey := getKey(pi)
	cancel, ok := m.probers[probeKey]
	if ok {
		cancel()
	}
}

func (m Manager) runProbe(ctx context.Context, p Prober) {
	ticker, stop := m.newTicker()

	defer stop()
	defer p.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			go m.sendProbe(ctx, p)
		}
	}
}

func (m Manager) sendProbe(ctx context.Context, p Prober) {
	pi := p.GetInfo()
	l := m.log.WithValues("service", pi.Service, "namespace", pi.Namespace, "name", pi.Name)

	o, err := m.hist.CurryWith(prometheus.Labels{
		"service":      pi.Service,
		"namespace":    pi.Namespace,
		"name":         pi.Name,
		"organization": pi.Organization,
		"ha":           strconv.FormatBool(pi.HighAvailable),
		"sla":          pi.ServiceLevel,
		"maintenance":  strconv.FormatBool(m.maintenanceStatus.IsMaintenanceRunning()),
	})
	if err != nil {
		l.Error(err, "failed to instanciate prometheus histogram")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ctx = withMaintenance(ctx, m.maintenanceStatus.IsMaintenanceRunning())

	start := time.Now()
	err = p.Probe(ctx)
	latency := time.Since(start)

	switch {
	case err == nil:
		o.With(
			prometheus.Labels{"reason": "success"},
		).Observe(latency.Seconds())
	case errors.Is(err, ErrTimeout) || errors.Is(ctx.Err(), context.DeadlineExceeded):
		l.V(0).Error(err, "Probe Timeout")
		o.With(
			prometheus.Labels{"reason": "fail-timeout"},
		).Observe(latency.Seconds())
	case errors.Is(ctx.Err(), context.Canceled):
		l.V(0).Info("Probe Canceled")
	default:
		l.V(0).Error(err, "Probe Failure")
		o.With(
			prometheus.Labels{"reason": "fail-unknown"},
		).Observe(latency.Seconds())
	}
}

func newTickerChan() (<-chan time.Time, func()) {
	ticker := time.NewTicker(time.Second)
	return ticker.C, ticker.Stop
}
