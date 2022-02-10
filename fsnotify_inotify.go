//go:build linux
// +build linux

package fsnotify

import "github.com/lestrrat-go/fsnotify/inotify"

func New() *Watcher {
	return Create(inotify.New())
}
