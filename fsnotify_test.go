package fsnotify_test

import (
	"context"

	"github.com/lestrrat-go/fsnotify"
)

func ExampleInotify() {
	driver := inotify.New()
	watcher := fsnotify.New(driver)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watcher.Run(ctx)

	watcher.Add("/foo/bar/baz")

	for watcher.Next() {
		ev := watcher.Event()

		switch ev.Type {

		}
	}
}
