//go:build freebsd || openbsd || netbsd || dragonfly || darwin
// +build freebsd openbsd netbsd dragonfly darwin

package fsnotify

// UNIMPLEMENTED
import (
	kqueue "github.com/lestrrat-go/fsnotify-kqueue"
)

func init() {
	DefaultDriverFunc = kqueue.New
}

