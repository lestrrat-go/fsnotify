package api

import (
	"context"
)

// Driver is the interface that must be implemented by the
// underlying fsnotify implementation.
type Driver interface {
	// Add adds a new watch target to the driver.
	Add(string, ...CommandOption) error

	// Remove removes a watch target.
	Remove(string, ...CommandOption) error

	// Run starts the driver's processing of the entries.
	//
	// The second parameter is used to tell the caller (fsnotiy.Wathcer)
	// that the driver is ready to serve requests. When the driver is
	// ready, close the channel to tell the Watcher that it's ready.
	//
	// The third and fourht parameters are where events and errors
	// should be sent from the Driver
	Run(context.Context, chan struct{}, EventSink, ErrorSink)
}

// NullDriver exists to be plugged in when there are no other
// drivers to be used. fsnotify will not function, but your
// code will at least not die a horrible death.
type NullDriver struct{}

func (_ NullDriver) Add(_ string, _ ...CommandOption) error {
	return nil
}
func (_ NullDriver) Remove(_ string, _ ...CommandOption) error {
	return nil
}
func (_ NullDriver) Run(_ context.Context, _ EventSink, _ ErrorSink) {}
