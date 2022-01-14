package fsnotify

import (
	"context"
	"path/filepath"
	"sync"
)

type Op uint32

const (
	OpCreate Op = 1 << iota
	OpWrite
	OpRemove
	OpRename
	OpChmod
)

type Event struct {
	Name string // relative path to file/directory
	Op   Op     //Platform-independent bitmask
}

type ctrlCmdType int

const (
	cmdAddEntry = iota + 1
	cmdRemoveEntry
	cmdEvent
)

type ctrlCmd struct {
	Type int
	Arg  interface{}
}

type Watcher struct {
	// The driver object behind this watcher.
	driver Driver

	// Hold the unique names of watch targets
	targets map[string]struct{}

	// list of commands that yet to be passed to the main Watch() goroutine
	pending []*ctrlCmd

	// list of unhandled events
	events []*Event

	control chan *ctrlCmd

	muPending *sync.RWMutex
	muEvents  *sync.RWMutex
	cond      *sync.Cond
}

// Create creates a new Watcher using the specified Driver.
func Create(d Driver) *Watcher {
	var muEvents sync.RWMutex
	var muPending sync.RWMutex
	return &Watcher{
		cond:      sync.NewCond(&muPending),
		control:   make(chan *ctrlCmd, 1),
		driver:    d,
		muEvents:  &muEvents,
		muPending: &muPending,
		targets:   make(map[string]struct{}),
	}
}

func (w *Watcher) Driver() Driver {
	return w.driver
}

func (w *Watcher) addCmd(cmd *ctrlCmd) {
	w.muPending.Lock()
	w.pending = append(w.pending, cmd)
	w.muPending.Unlock()
	// Send a signal to the goroutine that is listening for
	// update requests. It is important to make this a sync.Cond
	// object such that it has no effect even in the absense
	// of the background goroutine, make it non-blocking, and
	// to allow us to repeatedly perform this operation
	// (when channels close, they "work" only once. when we
	// send to channels, they may block)
	w.cond.Signal()
}

func (w *Watcher) Add(fn string) {
	_, ok := w.targets[fn]
	if !ok {
		w.targets[fn] = struct{}{}
		w.addCmd(&ctrlCmd{
			Type: cmdAddEntry,
			Arg:  fn,
		})
	}
}

func (w *Watcher) Remove(fn string) {
	_, ok := w.targets[fn]
	if ok {
		delete(w.targets, fn)
		w.addCmd(&ctrlCmd{
			Type: cmdRemoveEntry,
			Arg:  fn,
		})
	}

}

// ErrorSink is the destination where errors are reported to
type ErrorSink interface {
	// Error accepts an error that occurred during the execution
	// of `Watch()`. It must be non-blocking.
	Error(error)
}

func (w *Watcher) processPendingCmds(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		}

		w.muPending.Lock()
		for len(w.pending) == 0 {
			w.cond.Wait()
		}
		w.muPending.Unlock()

		// w.pending is populated. draing the queue
		for {
			w.muPending.Lock()
			l := len(w.pending)
			if l == 0 {
				w.muPending.Unlock()
				break
			}
			cmd := w.pending[0]
			w.pending = w.pending[1:]
			w.muPending.Unlock()

			select {
			case <-ctx.Done():
				return
			case w.control <- cmd:
			}
		}
	}
}

type nilSink struct{}

func (nilSink) Event(*Event) {}
func (nilSink) Error(error) {}

// EventSink is the destination where each Driver should send events to.
type EventSink interface {
	Event(*Event)
}

// Watch starts the watcher. By default it watches in the foreground,
// therefore if you would like this to run in the background you should
// execute it as a separate goroutine.
func (w *Watcher) Watch(ctx context.Context, options ...WatchOption) {
	// Unpack the options.
	var errSink ErrorSink = nilSink{}
	var evSink EventSink = nilSink{}
	for _, option := range options {
		switch option.Ident() {
		case identErrorSink{}:
			errSink = option.Value().(ErrorSink)
		case identEventSink{}:
			evSink = option.Value().(EventSink)
		}
	}

	// This is used to notify THIS goroutine about user
	// commands being queued.
	go w.processPendingCmds(ctx)
	// Make sure to wake up the above goroutine once when we exit,
	// so it can clean after itself
	defer w.cond.Signal()

	// Let the driver do its thing, and watch the events.
	// The second argument is the data sink
	go w.driver.Run(ctx, evSink, errSink)

	// While the driver does its thing, we process user commands.
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-w.control:
			if err := w.handleControlCmd(ctx, cmd); err != nil {
				errSink.Error(err)
			}
		}
	}
}

func (w *Watcher) handleControlCmd(ctx context.Context, cmd *ctrlCmd) error {
	switch cmd.Type {
	case cmdAddEntry:
		//nolint:forcetypeassert
		name := cmd.Arg.(string)
		name = filepath.Clean(name)
		return w.driver.Add(name)
	case cmdRemoveEntry:
		//nolint:forcetypeassert
		name := cmd.Arg.(string)
		name = filepath.Clean(name)
		return w.driver.Remove(name)
	default:
		//nolint:forcetypeassert
		ev := cmd.Arg.(*Event)
		// add the event to the events queue
		w.muEvents.Lock()
		w.events = append(w.events, ev)
		w.muEvents.Unlock()
		return nil
	}
}
