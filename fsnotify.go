package fsnotify

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/lestrrat-go/fsnotify/api"
)

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
	events []api.Event

	control chan *ctrlCmd

	muPending *sync.RWMutex
	muEvents  *sync.RWMutex
	muTargets *sync.RWMutex
	cond      *sync.Cond
}

// Create creates a new Watcher using the specified Driver.
func Create(d Driver) *Watcher {
	var muPending sync.RWMutex
	return &Watcher{
		cond:      sync.NewCond(&muPending),
		control:   make(chan *ctrlCmd, 1),
		driver:    d,
		muEvents:  &sync.RWMutex{},
		muPending: &muPending,
		muTargets: &sync.RWMutex{},
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
	w.muTargets.Lock()
	_, ok := w.targets[fn]
	if !ok {
		w.targets[fn] = struct{}{}
	}
	w.muTargets.Unlock()

	if !ok {
		w.add(fn)
	}
}

// factored out so that it can be used elsewhere
func (w *Watcher) add(fn string) {
	w.addCmd(&ctrlCmd{
		Type: cmdAddEntry,
		Arg:  fn,
	})
}

func (w *Watcher) Remove(fn string) {
	w.muTargets.Lock()
	_, ok := w.targets[fn]
	if ok {
		delete(w.targets, fn)
	}
	w.muTargets.Unlock()

	if ok {
		w.addCmd(&ctrlCmd{
			Type: cmdRemoveEntry,
			Arg:  fn,
		})
	}
}

func (w *Watcher) processPendingCmds(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
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

func (nilSink) Event(api.Event) {}
func (nilSink) Error(error)     {}

func (w *Watcher) clearPending() {
	w.muPending.Lock()
	w.pending = nil
	w.muPending.Unlock()
}

// Watch starts the watcher. By default it watches in the foreground,
// therefore if you would like this to run in the background you should
// execute it as a separate goroutine.
func (w *Watcher) Watch(ctx context.Context, options ...WatchOption) {
	defer w.clearPending()

	// Unpack the options.
	var errSink api.ErrorSink = nilSink{}
	var evSink api.EventSink = nilSink{}
	for _, option := range options {
		switch option.Ident() {
		case identErrorSink{}:
			errSink = option.Value().(api.ErrorSink)
		case identEventSink{}:
			evSink = option.Value().(api.EventSink)
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
	ready := make(chan struct{})
	go w.driver.Run(ctx, ready, evSink, errSink)

	<-ready

	// re-add targets. The driver could have been restarted
	// after it has been initialized once. This process assures that the
	// user doesn't have to re-add everything, while keeping the API
	// completely detached from how the Driver stores this data
	w.muTargets.Lock()
	for fn := range w.targets {
		w.add(fn)
	}
	w.muTargets.Unlock()

	// Let the command queue know that we're ready, just to make sure
	// everything that was done while we were idle is flushed
	w.cond.Signal()

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
		return w.driver.Add(ctx, name)
	case cmdRemoveEntry:
		//nolint:forcetypeassert
		name := cmd.Arg.(string)
		name = filepath.Clean(name)
		return w.driver.Remove(ctx, name)
	default:
		//nolint:forcetypeassert
		ev := cmd.Arg.(api.Event)
		// add the event to the events queue
		w.muEvents.Lock()
		w.events = append(w.events, ev)
		w.muEvents.Unlock()
		return nil
	}
}
