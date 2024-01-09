package probes

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	maintenancecontroller "github.com/vshn/appcat/v4/pkg/sliexporter/maintenance_controller"
)

func TestManger_Simple(t *testing.T) {
	t.Parallel()

	m := NewManager(logr.Discard(), &maintenancecontroller.MaintenanceReconciler{})
	observer := &fakeObserveVec{
		observations: &[]observation{},
		mu:           &sync.Mutex{},
		curried: observation{
			labels: map[string]string{},
		},
	}
	m.hist = observer
	p := newFakeProbe("fake", "bar", "foo")
	m.newTicker = func() (<-chan time.Time, func()) {
		return p.ticker, func() {}
	}

	m.StartProbe(p)
	p.tick(nil)
	p.tick(nil)
	p.tick(errors.New("ups"))
	p.tick(nil)
	p.tick(nil)

	assert.Eventually(t, func() bool {
		return p.getCount() == 5
	}, time.Second, 10*time.Millisecond)
	assert.EqualValues(t, 5, p.getCount())

	m.StopProbe(p.GetInfo())
	assert.EqualValues(t, 1, observer.countWithout("reason", "success"))

	p.tick(nil)
	assert.Never(t, func() bool {
		return p.getCount() > 5
	}, 250*time.Millisecond, 10*time.Millisecond)

}

func TestManger_Multi(t *testing.T) {
	t.Parallel()

	m := NewManager(logr.Discard(), &maintenancecontroller.MaintenanceReconciler{})
	tickerChan := make(chan chan time.Time)
	m.newTicker = func() (<-chan time.Time, func()) {
		return <-tickerChan, func() {}
	}
	observer := &fakeObserveVec{
		observations: &[]observation{},
		mu:           &sync.Mutex{},
		curried: observation{
			labels: map[string]string{},
		},
	}
	m.hist = observer

	pa := newFakeProbe("fake", "foo", "alice")
	pb := newFakeProbe("fake", "foo", "bob")

	m.StartProbe(pa)
	tickerChan <- pa.ticker
	m.StartProbe(pb)
	tickerChan <- pb.ticker
	pa.tick(nil)
	pb.tick(errors.New("Failure"))
	pa.tick(nil)
	pb.tick(nil)
	pa.tick(nil)
	pa.tick(nil)
	pa.tick(errors.New("Failure"))
	pa.tick(errors.New("Failure"))
	pb.tick(nil)
	pa.tick(errors.New("Failure"))
	pb.tick(nil)
	pa.tick(nil)
	pa.tick(nil)

	assert.Eventually(t, func() bool {
		return pa.getCount() == 9
	}, time.Second, 10*time.Millisecond)
	assert.EqualValues(t, 9, pa.getCount())
	m.StopProbe(ProbeInfo{
		Service:   "fake",
		Namespace: "foo",
		Name:      "alice",
	})

	pb.tick(nil)
	pb.tick(nil)
	pa.tick(errors.New("Failure"))

	assert.Eventually(t, func() bool {
		return pb.getCount() == 6
	}, time.Second, 10*time.Millisecond)
	assert.EqualValues(t, 6, pb.getCount())
	m.StopProbe(pb.GetInfo())

	assert.EqualValues(t, 4, observer.countWithout("reason", "success"))
	assert.EqualValues(t, 6, observer.countWith("name", "bob"))
	assert.EqualValues(t, 9, observer.countWith("name", "alice"))

	assert.Never(t, func() bool {
		return pa.getCount() > 9
	}, 250*time.Millisecond, 10*time.Millisecond)

}

func newFakeProbe(service, namespace, name string) *fakeProbe {
	return &fakeProbe{
		info: ProbeInfo{
			Service:      service,
			Name:         name,
			Namespace:    namespace,
			Organization: "foo",
		},
		results: make(chan error, 10),
		ticker:  make(chan time.Time, 10),
	}

}

func (f *fakeProbe) tick(res error) {
	f.results <- res
	f.ticker <- f.current
	f.current = f.current.Add(5 * time.Second)
}

type fakeProbe struct {
	info ProbeInfo

	results chan error
	ticker  chan time.Time
	current time.Time
	count   uint64
}

// Close implements Prober
func (p *fakeProbe) Close() error {
	return nil
}

// GetInfo implements Prober
func (p *fakeProbe) GetInfo() ProbeInfo {
	return p.info
}

// Probe implements Prober
func (f *fakeProbe) Probe(ctx context.Context) error {
	atomic.AddUint64(&f.count, 1)
	err := <-f.results
	return err
}

func (f *fakeProbe) getCount() uint64 {
	return atomic.LoadUint64(&f.count)
}

type observation struct {
	labels prometheus.Labels
	value  float64
}

type fakeObserveVec struct {
	observations *[]observation
	mu           *sync.Mutex

	curried observation
}

func (f *fakeObserveVec) countWith(key, val string) int {
	count := 0
	for _, o := range *f.observations {
		if o.labels[key] == val {
			count++
		}
	}
	return count
}
func (f *fakeObserveVec) countWithout(key, val string) int {
	count := 0
	for _, o := range *f.observations {
		if o.labels[key] != val {
			count++
		}
	}
	return count
}

// CurryWith implements prometheus.ObserverVec
func (f *fakeObserveVec) CurryWith(l prometheus.Labels) (prometheus.ObserverVec, error) {
	res := f.with(l)
	return res, nil
}

// With implements prometheus.ObserverVec
func (f *fakeObserveVec) With(l prometheus.Labels) prometheus.Observer {
	return f.with(l)
}

func (f *fakeObserveVec) with(l prometheus.Labels) *fakeObserveVec {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := fakeObserveVec{
		observations: f.observations,
		mu:           f.mu,
	}
	res.curried = f.observationWith(f.curried, l)
	return &res
}

func (f *fakeObserveVec) observationWith(obs observation, l prometheus.Labels) observation {
	//log.Printf("observationWith: %v prev %v\n", l, f.curried.labels)
	res := observation{
		labels: prometheus.Labels{},
	}
	for key, val := range f.curried.labels {
		res.labels[key] = val
	}
	for key, val := range l {
		res.labels[key] = val
	}
	return res
}

func (f *fakeObserveVec) Observe(v float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obs := observation{
		labels: map[string]string{},
		value:  v,
	}
	for key, val := range f.curried.labels {
		obs.labels[key] = val
	}
	*f.observations = append(*f.observations, obs)
}

// Collect implements prometheus.ObserverVec
func (fakeObserveVec) Collect(chan<- prometheus.Metric) {
	panic("unimplemented")
}

// Describe implements prometheus.ObserverVec
func (fakeObserveVec) Describe(chan<- *prometheus.Desc) {
	panic("unimplemented")
}

// GetMetricWith implements prometheus.ObserverVec
func (fakeObserveVec) GetMetricWith(prometheus.Labels) (prometheus.Observer, error) {
	panic("unimplemented")
}

// GetMetricWithLabelValues implements prometheus.ObserverVec
func (fakeObserveVec) GetMetricWithLabelValues(lvs ...string) (prometheus.Observer, error) {
	panic("unimplemented")
}

// MustCurryWith implements prometheus.ObserverVec
func (fakeObserveVec) MustCurryWith(prometheus.Labels) prometheus.ObserverVec {
	panic("unimplemented")
}

// WithLabelValues implements prometheus.ObserverVec
func (fakeObserveVec) WithLabelValues(...string) prometheus.Observer {
	panic("unimplemented")
}
