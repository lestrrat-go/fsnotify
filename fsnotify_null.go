//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !solaris && !windows
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!solaris,!windows

package fsnotify

func New() *Watcher {
	return Create(NewNullDriver())
}
