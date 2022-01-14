//go:build linux
// +build linux

package fsnotify

import "github.com/lestrrat-go/fsnotify-inotify"

// New creates a new Watcher using the default underlying
// implementation.
func New() *Watcher {
	return Create(inotify.New())
}
