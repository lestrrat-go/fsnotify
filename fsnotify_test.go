package fsnotify_test

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	"github.com/lestrrat-go/fsnotify"
	"github.com/lestrrat-go/fsnotify/api"
	"github.com/stretchr/testify/assert"
)

func TestWatcher(t *testing.T) {
	dir, err := ioutil.TempDir("", "fsnotify-test-*")
	if !assert.NoError(t, err, `ioutil.TempDir should succeed`) {
		return
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	f, err := ioutil.TempFile(dir, "test-watcher-*")
	if !assert.NoError(t, err, `ioutil.TempFile should succeed`) {
		return
	}
	t.Cleanup(func() { f.Close() })

	os.Chmod(f.Name(), 0600)

	watcher := fsnotify.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := make(chan api.Event)
	errCh := make(chan error)

	go watcher.Watch(ctx,
		fsnotify.WithEventSink(fsnotify.ChannelEventSink(eventCh)),
		fsnotify.WithErrorSink(fsnotify.ChannelErrorSink(errCh)),
	)

	watcher.Add(f.Name())

	// TODO: haven't checked how Create shold work
	// Write, Chmod, Rename, Remove
	time.AfterFunc(500*time.Millisecond, func() {
		f.Write([]byte(`Hello, World!`))
		f.Sync()
	})

	time.AfterFunc(time.Second, func() {
		os.Chmod(f.Name(), 0700)
	})

	time.AfterFunc(1500*time.Millisecond, func() {
		os.Rename(f.Name(), f.Name()+`renamed`)
	})

	time.AfterFunc(2000*time.Millisecond, func() {
		os.Rename(f.Name()+`renamed`, f.Name())
	})

	time.AfterFunc(2500*time.Millisecond, func() {
		assert.NoError(t, os.Remove(f.Name()), `os.Remove(%q) should succeed`, f.Name())
	})

	time.AfterFunc(3000*time.Millisecond, cancel)

	var events []api.Event
	var errors []error
	go func(ctx context.Context) {
		for {
			select {
			case err := <-errCh:
				errors = append(errors, err)
			case ev := <-eventCh:
				events = append(events, ev)
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	<-ctx.Done()

	if !assert.Len(t, errors, 0, `there should be no errors`) {
		t.Logf("%#v", errors)
		return
	}

	// events should be WRITE,CHMOD,RENAME,RENAME,REMOVE
	expected := []api.Op{
		api.OpWrite,
		api.OpChmod,
		api.OpRename,
		api.OpRename,
	}

	if !assert.Len(t, events, len(expected), `number of expected events should match`) {
		return
	}

	for i, ev := range events {
		if !assert.True(t, ev.Mask().IsSet(expected[i]), `mask should have %s enabled for events[%d]`, expected[i], i) {
			return
		}
	}
}

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
