package wiface

import (
	"sync"
)

type endpointSequencePool struct {
	pool sync.Pool
}

func newEndpointSequencePool() *endpointSequencePool {
	return &endpointSequencePool{
		pool: sync.Pool{
			New: func() interface{} {
				return &endpointSequence{}
			},
		},
	}
}

func (p *endpointSequencePool) get() *endpointSequence {
	return p.pool.Get().(*endpointSequence)
}

func (p *endpointSequencePool) put(e *endpointSequence) {
	e.id = 0
	e.ts = 0
	p.pool.Put(e)
}
