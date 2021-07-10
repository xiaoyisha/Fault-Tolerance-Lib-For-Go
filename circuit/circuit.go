package circuit

import (
	"Fault-Tolerance-Lib-For-Go/config"
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
}

var (
	circuitBreakersMutex *sync.RWMutex
	circuitBreakers      map[string]*CircuitBreaker
)

func init() {
	circuitBreakersMutex = &sync.RWMutex{}
	circuitBreakers = make(map[string]*CircuitBreaker)
}

// TODO
func GetCircuitBreaker(name string) (*CircuitBreaker, bool, error) {
	return circuitBreakers[name], false, nil
}

func (circuitBreaker *CircuitBreaker) switchForceOpen(forceOpen bool) error {
	circuitBreaker, _, err := GetCircuitBreaker(circuitBreaker.Name)
	if err != nil {
		return err
	}
	circuitBreaker.forceOpen = forceOpen
	return nil
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

	// TODO Metrics

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
			log.Printf("hystrix-go: allowing single test to possibly close circuit %v", circuitBreaker.Name)
		}
		return swapped
	}

	return false
}
