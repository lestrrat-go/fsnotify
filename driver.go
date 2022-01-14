package fsnotify

import "context"

// Driver is the interface that must be implemented by the
// underlying fsnotify implementation.
type Driver interface {
	// Add adds a new watch target to the driver.
	Add(string) error

	// Remove removes a watch target.
	Remove(string) error

	// Run starts the driver's processing of the entries
	Run(context.Context, EventSink, ErrorSink)
}
