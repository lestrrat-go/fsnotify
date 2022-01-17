package fsnotify_test

import (
	"context"
	"log"

	"github.com/lestrrat-go/fsnotify"
	"github.com/lestrrat-go/fsnotify/api"
)

func ExampleInotify() {
	watcher := fsnotify.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := make(chan api.Event)
	errCh := make(chan error)

	go watcher.Watch(ctx,
		// The event and error sinks can be of any type, but here
		// we're using channels
		fsnotify.WithEventSink(fsnotify.ChannelEventSink(eventCh)),
		fsnotify.WithErrorSink(fsnotify.ChannelErrorSink(errCh)),
	)

	watcher.Add("go.mod")

	for {
		select {
		case err := <-errCh:
			log.Printf("%s", err)
		case ev := <-eventCh:
			log.Printf("%#v", ev)
		case <-ctx.Done():
			log.Printf("done")
			return
		}
	}
}
