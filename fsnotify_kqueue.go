//go:build freebsd || openbsd || netbsd || dragonfly || darwin
// +build freebsd openbsd netbsd dragonfly darwin

package fsnotify

import "github.com/lestrrat-go/fsnotify-kqueue"

// New creates a new Watcher using the default underlying
// implementation.
func New() *Watcher {
	return Create(kqueue.New())
}

