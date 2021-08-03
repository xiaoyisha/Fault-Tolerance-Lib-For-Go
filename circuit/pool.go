package circuit

import (
	"Perseus/config"
	"Perseus/rolling"
	"sync"
)

type ExecutorPool struct {
	Name    string
	MaxReq  int
	Tickets chan *struct{}
	Metrics *poolMetrics
}

func NewExecutorPool(name string) *ExecutorPool {
	p := &ExecutorPool{}
	p.Name = name
	p.MaxReq = config.GetCircuitConfig(name).MaxConcurrentRequests
	p.Tickets = make(chan *struct{}, p.MaxReq)
	for i := 0; i < p.MaxReq; i++ {
		p.Tickets <- &struct{}{}
	}
	p.Metrics = newPoolMetrics(name)

	return p
}

// ReturnTicket return ticket to the pool
func (p *ExecutorPool) ReturnTicket(ticket *struct{}) {
	if ticket == nil {
		return
	}
	p.Metrics.Updates <- poolMetricsUpdate{
		activeCount: p.ActiveCount(),
	}

	p.Tickets <- ticket
}

// ActiveCount number of threads that are active in the pool
func (p *ExecutorPool) ActiveCount() int {
	return p.MaxReq - len(p.Tickets)
}

// pool metrics
type poolMetrics struct {
	Mutex   *sync.RWMutex
	Updates chan poolMetricsUpdate

	Name              string
	MaxActiveRequests *rolling.Number
	Executed          *rolling.Number
}

type poolMetricsUpdate struct {
	activeCount int
}

func newPoolMetrics(name string) *poolMetrics {
	m := &poolMetrics{}
	m.Name = name
	m.Updates = make(chan poolMetricsUpdate)
	m.Mutex = &sync.RWMutex{}

	m.Reset()

	go m.Monitor()

	return m
}

func (m *poolMetrics) Reset() {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	m.MaxActiveRequests = rolling.NewNumber()
	m.Executed = rolling.NewNumber()
}

func (m *poolMetrics) Monitor() {
	for u := range m.Updates {
		m.Mutex.RLock()

		m.Executed.Increment(1)
		m.MaxActiveRequests.UpdateMax(float64(u.activeCount))

		m.Mutex.RUnlock()
	}
}
