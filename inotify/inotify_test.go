//go:build linux
// +build linux

package inotify_test

import (
	"github.com/lestrrat-go/fsnotify/api"
	"github.com/lestrrat-go/fsnotify/inotify"
)

// Sanity
var _ api.Driver = &inotify.Driver{}
