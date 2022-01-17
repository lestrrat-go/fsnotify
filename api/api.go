// Package api defines the interfaces and data structures that are
// used by both the main fsnotify interface and the fsnotify drivers.
//
// This package has been separated into its own package to avoid
// cyclic imports between the fsnotify and driver packages.

package api

type Op uint32

const (
	OpCreate uint32 = 1 << iota
	OpWrite
	OpRemove
	OpRename
	OpChmod
)

func (op *Op) Mark(v uint32) {
	*((*uint32)(op)) |= v
}

func (op *Op) Unmark(v uint32) {
	*((*uint32)(op)) ^= v
}

func (op *Op) IsSet(v uint32) bool {
	return *((*uint32)(op))&v != 0
}

// Event represents a file system event. It is an interface
// because driver implementation may want to extend the event
// and store extra information
type Event interface {
	// Name returns the name of file that caused this event
	Name() string

	// Op returns a bitmask of operation(s) that caused this event
	Op() uint32
}

type event struct {
	name string
	op   Op
}

func NewEvent(name string, op Op) Event {
	return &event{name: name, op: op}
}

func (ev *event) Name() string {
	return ev.name
}

func (ev *event) Op() uint32 {
	return uint32(ev.op)
}

// EventSink is the destination where each Driver should send events to.
type EventSink interface {
	Event(Event)
}

// ErrorSink is the destination where errors are reported to
type ErrorSink interface {
	// Error accepts an error that occurred during the execution
	// of `Watch()`. It must be non-blocking.
	Error(error)
}
