package api

import (
	"context"
	"sync"
)

// Command represents a request sent from the fsnotify watcher to the driver.
// The driver is (presumably) running asynchronously wrt to the watcher, and
// thus operations that modify the state of the driver are passed via channels.
//
// The Command object is the object that gets passed from the watcher goroutine
// to the driver goroutine.
type Command struct {
	// the type of request. Values in this field will mean different things
	// depending on the driver
	Type int

	// The payload used to fulfill the operation. Maybe nil depending on the
	// driver and the operation
	Payload interface{}

	// A channel where errors can be reported back to the watcher goroutine.
	Reply chan error
}

type CommandQueueEgressChooser interface {
	Choose(*Command) chan *Command
}

type CommandQueueEgressChooseFunc func(*Command) chan *Command

func (fn CommandQueueEgressChooseFunc) Choose(cmd *Command) chan *Command {
	return fn(cmd)
}

type CommandQueue struct {
	mu      *sync.Mutex
	cond    *sync.Cond
	pending []*Command
	chooser CommandQueueEgressChooser
}

func NewCommandQueue(chooser CommandQueueEgressChooser) *CommandQueue {
	mu := &sync.Mutex{}
	return &CommandQueue{
		mu:      mu,
		cond:    sync.NewCond(mu),
		chooser: chooser,
	}
}

func (q *CommandQueue) Append(cmd *Command) {
	q.mu.Lock()
	q.pending = append(q.pending, cmd)
	q.mu.Unlock()

	q.cond.Signal()
}

const bufferProcessSize = 32

func (q *CommandQueue) Drain(ctx context.Context) {
	pending := make([]*Command, bufferProcessSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		q.cond.L.Lock()
		for len(q.pending) <= 0 {
			q.cond.Wait()

			select {
			case <-ctx.Done():
				q.cond.L.Unlock()
				return
			default:
			}
		}

		l := len(q.pending)
		if l < bufferProcessSize {
			pending = pending[:l]
		} else {
			pending = pending[:bufferProcessSize]
		}

		n := copy(pending, q.pending)
		q.pending = q.pending[:n]

		q.cond.L.Unlock()

		for _, v := range q.pending {
			egress := q.chooser.Choose(v)
			select {
			case <-ctx.Done():
				return
			case egress <- v:
			}
		}
	}
}

func (q *CommandQueue) SendCmd(cmd *Command, options ...CommandOption) error {
	var ack bool
	for _, option := range options {
		//nolint:forcetypeassert
		ident := option.Ident()
		switch {
		case IsAck(ident):
			ack = option.Value().(bool)
		}
	}

	if ack {
		cmd.Reply = make(chan error, 1)
	}

	q.Append(cmd)
	if ack {
		return <-cmd.Reply
	}
	return nil
}
