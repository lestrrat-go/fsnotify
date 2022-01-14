package fsnotify_test

import (
	"context"

	"github.com/lestrrat-go/fsnotify"
	"github.com/lestrrat-go/fsnotify-inotify"
)

func ExampleInotify() {
	driver := inotify.New()
	watcher := fsnotify.New(driver)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := make(chan *fsnotify.Event)
	errCh := make(chan error)

	go watcher.Watch(ctx,
		// The event and error sinks can be of any type, but here
		// we're using channels
		fsnotify.WithEventSink(fsnotify.ChannelEventSink(eventCh)),
		fsnotify.WithErrorSink(fsnotify.ChannelErrorSink(errCh)),
	)

	watcher.Add("/foo/bar/baz")

	for {
		select {
		case err := <-errCh:
		case ev := <-eventCh:
		case <-ctx.Done():
			return
		}
	}

}
