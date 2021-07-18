package metrics

import (
	"Fault-Tolerance-Lib-For-Go/config"
	"Fault-Tolerance-Lib-For-Go/rolling"
	"sync"
	"time"
)

type CommandExecution struct {
	Types            []string      `json:"types"`
	Start            time.Time     `json:"start_time"`
	RunDuration      time.Duration `json:"run_duration"`
	ConcurrencyInUse float64       `json:"concurrency_inuse"`
}

type MetricExchange struct {
	Name    string
	Updates chan *CommandExecution
	Mutex   *sync.RWMutex

	metricCollectors []MetricCollector
}

func NewMetricExchange(name string) *MetricExchange {
	m := &MetricExchange{}
	m.Name = name

	m.Updates = make(chan *CommandExecution, 2000)
	m.Mutex = &sync.RWMutex{}
	m.metricCollectors = Registry.InitializeMetricCollectors(name)
	m.Reset()

	go m.Monitor()

	return m
}

// The Default Collector function will panic if collectors are not setup to specification.
func (m *MetricExchange) DefaultCollector() *DefaultMetricCollector {
	if len(m.metricCollectors) < 1 {
		panic("No Metric Collectors Registered.")
	}
	collection, ok := m.metricCollectors[0].(*DefaultMetricCollector)
	if !ok {
		panic("Default metric collector is not registered correctly. The default metric collector must be registered first.")
	}
	return collection
}

func (m *MetricExchange) Monitor() {
	for update := range m.Updates {
		// we only grab a read lock to make sure Reset() isn't changing the numbers.
		m.Mutex.RLock()

		totalDuration := time.Since(update.Start)
		wg := &sync.WaitGroup{}
		for _, collector := range m.metricCollectors {
			wg.Add(1)
			go m.IncrementMetrics(wg, collector, update, totalDuration)
		}
		wg.Wait()

		m.Mutex.RUnlock()
	}
}

func (m *MetricExchange) IncrementMetrics(wg *sync.WaitGroup, collector MetricCollector, update *CommandExecution, totalDuration time.Duration) {
	// granular metrics
	r := MetricResult{
		Attempts:         1,
		TotalDuration:    totalDuration,
		RunDuration:      update.RunDuration,
		ConcurrencyInUse: update.ConcurrencyInUse,
	}

	switch update.Types[0] {
	case "success":
		r.Successes = 1
	case "failure":
		r.Failures = 1
		r.Errors = 1
	case "rejected":
		r.Rejects = 1
		r.Errors = 1
	case "short-circuit":
		r.ShortCircuits = 1
		r.Errors = 1
	case "timeout":
		r.Timeouts = 1
		r.Errors = 1
	case "context_canceled":
		r.ContextCanceled = 1
	case "context_deadline_exceeded":
		r.ContextDeadlineExceeded = 1
	}

	if len(update.Types) > 1 {
		// fallback metrics
		if update.Types[1] == "fallback-success" {
			r.FallbackSuccesses = 1
		}
		if update.Types[1] == "fallback-failure" {
			r.FallbackFailures = 1
		}
	}

	collector.Update(r)

	wg.Done()
}

func (m *MetricExchange) Reset() {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	for _, collector := range m.metricCollectors {
		collector.Reset()
	}
}

func (m *MetricExchange) Requests() *rolling.Number {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()
	return m.requestsLocked()
}

func (m *MetricExchange) requestsLocked() *rolling.Number {
	return m.DefaultCollector().NumRequests()
}

func (m *MetricExchange) ErrorPercent(now time.Time) int {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()

	var errPct float64
	reqs := m.requestsLocked().Sum(now)
	errs := m.DefaultCollector().Errors().Sum(now)

	if reqs > 0 {
		errPct = (float64(errs) / float64(reqs)) * 100
	}

	return int(errPct + 0.5)
}

func (m *MetricExchange) IsHealthy(now time.Time) bool {
	return m.ErrorPercent(now) < config.GetCircuitConfig(m.Name).ErrorPercentThreshold
}

func MetricFailingPercent(p int) *MetricExchange {
	m := NewMetricExchange("")
	for i := 0; i < 100; i++ {
		t := "success"
		if i < p {
			t = "failure"
		}
		m.Updates <- &CommandExecution{Types: []string{t}}
	}

	// Updates needs to be flushed
	time.Sleep(100 * time.Millisecond)

	return m
}
