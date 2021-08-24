package Perseus

import (
	"Perseus/circuit"
	"Perseus/config"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type RunFunc func() error
type FallbackFunc func(error) error
type RunFuncC func(context.Context) error
type FallbackFuncC func(context.Context, error) error

// A CircuitError is an error which models various failure states of execution,
// such as the circuit being open or a timeout.
type CircuitError struct {
	Message string
}

func (e CircuitError) Error() string {
	return "Perseus: " + e.Message
}

// Command models the state used for a single execution on a circuit. "Perseus command" is commonly
// used to describe the pairing of your run/fallback functions with a circuit.
type Command struct {
	sync.Mutex

	name           string
	ticket         *struct{}
	ticketCond     *sync.Cond
	ticketGot      bool
	returnOnce     *sync.Once
	circuitBreaker *circuit.CircuitBreaker
	run            RunFuncC
	fallback       FallbackFuncC
	start          time.Time
	errChan        chan error
	finished       chan bool
	runDuration    time.Duration
	events         []string
}

var (
	// ErrMaxConcurrency occurs when too many of the same named command are executed at the same time.
	ErrMaxConcurrency = CircuitError{Message: "max concurrency"}
	// ErrCircuitOpen returns when an execution attempt "short circuits". This happens due to the circuit being measured as unhealthy.
	ErrCircuitOpen = CircuitError{Message: "circuit open"}
	// ErrTimeout occurs when the provided function takes too long to execute.
	ErrTimeout = CircuitError{Message: "timeout"}
)

func Go(name string, run RunFunc, fallback FallbackFunc) chan error {
	runC := func(context.Context) error {
		return run()
	}
	var fallbackC FallbackFuncC
	if fallback != nil {
		fallbackC = func(ctx context.Context, err error) error {
			return fallback(err)
		}
	}
	return GoC(context.Background(), name, runC, fallbackC)
}

// GoC runs your function while tracking the health of previous calls to it.
// If your function begins slowing down or failing repeatedly, we will block
// new calls to it for you to give the dependent service time to repair.
//
// Define a fallback function if you want to define some code to execute during outages.
func GoC(ctx context.Context, name string, run RunFuncC, fallback FallbackFuncC) chan error {
	cmd := &Command{
		name:       name,
		run:        run,
		ticketGot:  false,
		fallback:   fallback,
		start:      time.Now(),
		errChan:    make(chan error, 1),
		finished:   make(chan bool, 1),
		returnOnce: &sync.Once{},
	}
	cmd.ticketCond = sync.NewCond(cmd)

	circuitBreaker, _, err := circuit.GetCircuitBreaker(name)
	if err != nil {
		cmd.errChan <- err
		return cmd.errChan
	}
	cmd.circuitBreaker = circuitBreaker
	go cmd.firstGoroutine(ctx)
	go cmd.secondGoroutine(ctx)
	return cmd.errChan
}

func (c *Command) returnTicket() {
	c.Lock()
	// Avoid releasing before a ticket is acquired.
	for !c.ticketGot {
		c.ticketCond.Wait()
	}
	c.circuitBreaker.ExecutorPool.ReturnTicket(c.ticket)
	c.Unlock()
}

func (c *Command) reportEvent(eventType string) {
	c.Lock()
	defer c.Unlock()

	c.events = append(c.events, eventType)
}

func (c *Command) reportAllEvents() {
	err := c.circuitBreaker.ReportEvent(c.events, c.start, c.runDuration)
	if err != nil {
		log.Printf(err.Error())
	}
}

func (c *Command) firstGoroutine(ctx context.Context) {
	defer func() { c.finished <- true }()
	if !c.circuitBreaker.AllowRequest() {
		c.Lock()
		// It's safe for another goroutine to go ahead releasing a nil ticket.
		c.ticketGot = true
		c.ticketCond.Signal()
		c.Unlock()
		c.returnOnce.Do(func() {
			c.returnTicket()
			c.errorWithFallback(ctx, ErrCircuitOpen)
		})
		return
	}
	// As backends falter, requests take longer but don't always fail.
	//
	// When requests slow down but the incoming rate of requests stays the same, you have to
	// run more at a time to keep up. By controlling concurrency during these situations, you can
	// shed load which accumulates due to the increasing ratio of active commands to incoming requests.
	c.Lock()
	select {
	case c.ticket = <-c.circuitBreaker.ExecutorPool.Tickets:
		c.ticketGot = true
		c.ticketCond.Signal()
		c.Unlock()
	default:
		c.ticketGot = true
		c.ticketCond.Signal()
		c.Unlock()
		c.returnOnce.Do(func() {
			c.returnTicket()
			c.errorWithFallback(ctx, ErrMaxConcurrency)
		})
		return
	}
	runStart := time.Now()
	runErr := c.run(ctx)
	log.Printf("runErr: %v", runErr)

	c.returnOnce.Do(func() {
		c.runDuration = time.Since(runStart)
		c.returnTicket()
		log.Printf("runErr: %v", runErr)
		if runErr != nil {
			c.errorWithFallback(ctx, runErr)
			return
		}
		c.reportEvent("success")
		c.reportAllEvents()
	})
}

func (c *Command) secondGoroutine(ctx context.Context) {
	timer := time.NewTimer(config.GetCircuitConfig(c.name).Timeout)
	defer timer.Stop()

	select {
	case <-c.finished:
		// returnOnce has been executed in another goroutine
	case <-ctx.Done():
		c.returnOnce.Do(func() {
			c.returnTicket()
			c.errorWithFallback(ctx, ctx.Err())
		})
		return
	case <-timer.C:
		c.returnOnce.Do(func() {
			log.Printf("runErr: timeout")
			c.returnTicket()
			c.errorWithFallback(ctx, ErrTimeout)
		})
		return
	}
}

func (c *Command) errorWithFallback(ctx context.Context, err error) {
	eventType := "failure"
	if err == ErrCircuitOpen {
		eventType = "short-circuit"
	} else if err == ErrMaxConcurrency {
		eventType = "rejected"
	} else if err == ErrTimeout {
		eventType = "timeout"
	} else if err == context.Canceled {
		eventType = "context_canceled"
	} else if err == context.DeadlineExceeded {
		eventType = "context_deadline_exceeded"
	}

	c.reportEvent(eventType)
	fallbackErr := c.tryFallback(ctx, err)
	if fallbackErr != nil {
		log.Printf("fallbackErr: %v", fallbackErr)
		c.errChan <- fallbackErr
	}
	c.reportAllEvents()
}

func (c *Command) tryFallback(ctx context.Context, err error) error {
	if c.fallback == nil {
		return err
	}

	fallbackErr := c.fallback(ctx, err)
	if fallbackErr != nil {
		c.reportEvent("fallback-failure")
		return fmt.Errorf("fallback err: %v, run err: %v", fallbackErr, err)
	}
	c.reportEvent("fallback-success")

	return nil
}

// Do runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned, including Perseus circuit errors
func Do(name string, run RunFunc, fallback FallbackFunc) error {
	runC := func(ctx context.Context) error {
		return run()
	}
	var fallbackC FallbackFuncC
	if fallback != nil {
		fallbackC = func(ctx context.Context, err error) error {
			return fallback(err)
		}
	}
	return DoC(context.Background(), name, runC, fallbackC)
}

// DoC runs your function in a synchronous manner, blocking until either your function succeeds
// or an error is returned, including Perseus circuit errors
func DoC(ctx context.Context, name string, run RunFuncC, fallback FallbackFuncC) error {
	done := make(chan struct{}, 1)

	r := func(ctx context.Context) error {
		err := run(ctx)
		if err != nil {
			return err
		}

		done <- struct{}{}
		return nil
	}

	f := func(ctx context.Context, e error) error {
		err := fallback(ctx, e)
		if err != nil {
			return err
		}

		done <- struct{}{}
		return nil
	}

	var errChan chan error
	if fallback == nil {
		errChan = GoC(ctx, name, r, nil)
	} else {
		errChan = GoC(ctx, name, r, f)
	}

	select {
	case <-done:
		return nil
	case err := <-errChan:
		return err
	}
}
