package entity

import (
	"sync"
)

type messagePool struct {
	pool sync.Pool
}

var MessagePool = &messagePool{
	pool: sync.Pool{
		New: func() interface{} {
			return &Message{}
		},
	},
}

func (p *messagePool) Get(n int) *Message {
	m := p.pool.Get().(*Message)
	if m.Payload != nil {
		if cap(m.Payload.([]byte)) < n {
			m.Payload = make([]byte, n)
		} else {
			m.Payload = m.Payload.([]byte)[:n]
		}
	} else {
		m.Payload = make([]byte, n)
	}
	return m
}

func (p *messagePool) Put(m *Message) {
	m.Reset()
	p.pool.Put(m)
}
