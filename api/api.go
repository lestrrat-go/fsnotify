// Package api defines the interfaces and data structures that are
// used by both the main fsnotify interface and the fsnotify drivers.
//
// This package has been separated into its own package to avoid
// cyclic imports between the fsnotify and driver packages.

package api

import (
	"strconv"
	"strings"
)

type OpMask uint32
type Op uint32

const (
	OpCreate Op = 1 << iota
	OpWrite
	OpRemove
	OpRename
	OpChmod
)

func (op Op) String() string {
	switch op {
	case OpCreate:
		return "CREATE"
	case OpWrite:
		return "WRITE"
	case OpRemove:
		return "REMOVE"
	case OpRename:
		return "RENAME"
	case OpChmod:
		return "CHMOD"
	default:
		return "INVALID OP"
	}
}

func (mask *OpMask) Set(op Op) {
	*((*uint32)(mask)) |= uint32(op)
}

func (mask *OpMask) Unset(op Op) {
	*((*uint32)(mask)) ^= uint32(op)
}

func (mask OpMask) IsSet(op Op) bool {
	return uint32(mask)&uint32(op) != 0
}

func (mask OpMask) String() string {
	var builder strings.Builder

	for _, op := range []Op{OpCreate, OpRemove, OpWrite, OpRename, OpChmod} {
		if uint32(mask)&uint32(op) == 0 {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('|')
		}
		builder.WriteString(op.String())
	}
	return builder.String()
}

// Event represents a file system event. It is an interface
// because driver implementation may want to extend the event
// and store extra information
type Event interface {
	// Name returns the name of file that caused this event
	Name() string

	// Op is a compatibility layer for github.com/fsnotify/fsnotify.
	Op() OpMask

	// OpMask returns a bitmask of operation(s) that caused this event
	Mask() OpMask

	// String() returns the string representation in a human readable
	// format. Do not expect the string to be stable or parsable.
	String() string
}

type event struct {
	name string
	mask OpMask
}

func NewEvent(name string, mask OpMask) Event {
	return &event{name: name, mask: mask}
}

func (ev *event) Name() string {
	return ev.name
}

func (ev *event) Op() OpMask {
	return ev.Mask()
}

func (ev *event) Mask() OpMask {
	return ev.mask
}

func (ev *event) String() string {
	var builder strings.Builder
	builder.WriteString(strconv.Quote(ev.name))
	builder.WriteString(` [`)
	builder.WriteString(ev.mask.String())
	builder.WriteString(`]`)
	return builder.String()
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
