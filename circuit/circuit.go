package circuit

import (
	"Perseus/config"
	"Perseus/metrics"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreaker tracks whether requests should be attempted or rejected
// for each ExecutorPool according to the Health of the circuit
type CircuitBreaker struct {
	Name                   string
	open                   bool
	forceOpen              bool
	mutex                  *sync.RWMutex
	openedOrLastTestedTime int64
	ExecutorPool           *ExecutorPool
	Metrics                *metrics.MetricExchange
}

// A CircuitError is an error which models various failure states of execution,
// such as the circuit being open or a timeout.
type CircuitError struct {
	Message string
}

func (e CircuitError) Error() string {
	return e.Message
}

var (
	circuitBreakersMutex *sync.RWMutex
	circuitBreakers      map[string]*CircuitBreaker
)

func init() {
	circuitBreakersMutex = &sync.RWMutex{}
	circuitBreakers = make(map[string]*CircuitBreaker)
}

// NewCircuitBreaker creates a CircuitBreaker with associated Health
func NewCircuitBreaker(name string) *CircuitBreaker {
	c := &CircuitBreaker{}
	c.Name = name
	c.Metrics = metrics.NewMetricExchange(name)
	c.ExecutorPool = NewExecutorPool(name)
	c.mutex = &sync.RWMutex{}

	return c
}

func GetCircuitBreaker(name string) (*CircuitBreaker, bool, error) {
	circuitBreakersMutex.RLock()
	_, ok := circuitBreakers[name]
	if !ok {
		circuitBreakersMutex.RUnlock()
		circuitBreakersMutex.Lock()
		defer circuitBreakersMutex.Unlock()
		// because we released the rlock before we obtained the exclusive lock,
		// we need to double check that some other thread didn't beat us to
		// creation.
		if cb, ok := circuitBreakers[name]; ok {
			return cb, false, nil
		}
		circuitBreakers[name] = NewCircuitBreaker(name)
	} else {
		defer circuitBreakersMutex.RUnlock()
	}

	return circuitBreakers[name], !ok, nil
}

func (circuitBreaker *CircuitBreaker) SwitchForceOpen(forceOpen bool) error {
	circuitBreaker, _, err := GetCircuitBreaker(circuitBreaker.Name)
	if err != nil {
		return err
	}
	circuitBreaker.forceOpen = forceOpen
	return nil
}

// Flush purges all circuit and metric information from memory.
func Flush() {
	circuitBreakersMutex.Lock()
	defer circuitBreakersMutex.Unlock()

	for name, cb := range circuitBreakers {
		cb.Metrics.Reset()
		cb.ExecutorPool.Metrics.Reset()
		delete(circuitBreakers, name)
	}
}

// IsOpen is called before any Command execution. An "open" circuit
// means the command should be rejected
func (circuitBreaker *CircuitBreaker) IsOpen() bool {
	circuitBreaker.mutex.RLock()
	o := circuitBreaker.forceOpen || circuitBreaker.open
	circuitBreaker.mutex.RUnlock()
	if o {
		return true
	}

	if uint64(circuitBreaker.Metrics.Requests().Sum(time.Now())) < config.GetCircuitConfig(circuitBreaker.Name).RequestVolumeThreshold {
		return false
	}

	if !circuitBreaker.Metrics.IsHealthy(time.Now()) {
		// too many failures, open the circuit
		circuitBreaker.SetOpen()
		return true
	}

	return false
}

// AllowRequest is called before any Command execution.
// When the circuit is open, this call will occasionally return true to measure whether the external service
// has recovered.
func (circuitBreaker *CircuitBreaker) AllowRequest() bool {
	return !circuitBreaker.IsOpen() || circuitBreaker.allowSingleTest()

}

func (circuitBreaker *CircuitBreaker) allowSingleTest() bool {
	circuitBreaker.mutex.RLock()
	defer circuitBreaker.mutex.RUnlock()

	now := time.Now().UnixNano()
	openedOrLastTestedTime := circuitBreaker.openedOrLastTestedTime
	if circuitBreaker.open && now > openedOrLastTestedTime+config.GetCircuitConfig(circuitBreaker.Name).SleepWindow.Nanoseconds() {
		swapped := atomic.CompareAndSwapInt64(&circuitBreaker.openedOrLastTestedTime, openedOrLastTestedTime, now)
		if swapped {
			log.Printf("allowing single test to possibly close circuit %v", circuitBreaker.Name)
		}
		return swapped
	}

	return false
}

func (circuitBreaker *CircuitBreaker) SetOpen() {
	circuitBreaker.mutex.Lock()
	defer circuitBreaker.mutex.Unlock()

	if circuitBreaker.open {
		return
	}

	log.Printf("opening circuit %v", circuitBreaker.Name)

	circuitBreaker.openedOrLastTestedTime = time.Now().UnixNano()
	circuitBreaker.open = true
}

func (circuitBreaker *CircuitBreaker) setClose() {
	circuitBreaker.mutex.Lock()
	defer circuitBreaker.mutex.Unlock()

	if !circuitBreaker.open {
		return
	}

	log.Printf("closing circuit %v", circuitBreaker.Name)

	circuitBreaker.open = false
	circuitBreaker.Metrics.Reset()
}

// ReportEvent records command metrics for tracking recent error rates
func (circuitBreaker *CircuitBreaker) ReportEvent(eventTypes []string, start time.Time, runDuration time.Duration) error {
	if len(eventTypes) == 0 {
		return fmt.Errorf("no event types sent for metrics")
	}

	circuitBreaker.mutex.RLock()
	o := circuitBreaker.open
	circuitBreaker.mutex.RUnlock()
	if eventTypes[0] == "success" && o {
		circuitBreaker.setClose()
	}

	var concurrencyInUse float64
	if circuitBreaker.ExecutorPool.MaxReq > 0 {
		concurrencyInUse = float64(circuitBreaker.ExecutorPool.ActiveCount()) / float64(circuitBreaker.ExecutorPool.MaxReq)
	}

	select {
	case circuitBreaker.Metrics.Updates <- &metrics.CommandExecution{
		Types:            eventTypes,
		Start:            start,
		RunDuration:      runDuration,
		ConcurrencyInUse: concurrencyInUse,
	}:
	default:
		return CircuitError{Message: fmt.Sprintf("metrics channel (%v) is at capacity", circuitBreaker.Name)}
	}

	return nil
}
