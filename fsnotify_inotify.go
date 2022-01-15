//go:build linux
// +build linux

package fsnotify

// UNIMPLEMENTED
import (
	inotify "github.com/lestrrat-go/fsnotify-inotify"
)

func init() {
	DefaultDriverFunc = inotify.New
}
