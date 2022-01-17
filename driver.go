package fsnotify

import (
	"context"

	"github.com/lestrrat-go/fsnotify/api"
)

// Driver is the interface that must be implemented by the
// underlying fsnotify implementation.
type Driver interface {
	// Add adds a new watch target to the driver.
	Add(context.Context, string) error

	// Remove removes a watch target.
	Remove(context.Context, string) error

	// Run starts the driver's processing of the entries.
	//
	// The second parameter is used to tell the caller (fsnotiy.Wathcer)
	// that the driver is ready to serve requests. When the driver is
	// ready, close the channel to tell the Watcher that it's ready.
	//
	// The third and fourht parameters are where events and errors
	// should be sent from the Driver
	Run(context.Context, chan struct{}, api.EventSink, api.ErrorSink)
}
