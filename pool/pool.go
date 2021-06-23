package pool

import "Fault-Tolerance-Lib-For-Go/config"

type ExecutorPool struct {
	Name    string
	MaxReq  int
	Tickets chan *struct{}
}

func NewExecutorPool(name string) *ExecutorPool {
	p := &ExecutorPool{}
	p.Name = name
	p.MaxReq = config.GetCircuitConfig(name).MaxConcurrentRequests
	for i := 0; i < p.MaxReq; i++ {
		p.Tickets <- &struct{}{}
	}
	return p
}

// ReturnTicket return ticket to the pool
func (p *ExecutorPool) ReturnTicket(ticket *struct{}) {
	if ticket == nil {
		return
	}

	p.Tickets <- ticket
}

// ActiveCount number of threads that are active in the pool
func (p *ExecutorPool) ActiveCount() int {
	return p.MaxReq - len(p.Tickets)
}
