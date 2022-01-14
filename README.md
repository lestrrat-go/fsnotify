fsnotify
========

Currently, this is a study in how I would go about implementing a `fsnotify`
implementation, where the backend drivers can be pluggable.

The design differs significantly from https://github.com/fsnotify/fsnotify,
in that this package:

1. Uses `contet.Context`
2. Allows user to choose where to send their data via use of abstract Sinks
3. Does not implicitly use goroutines, but rather asks the user to choose by providing a `w.Watch()` method that can be called as `go w.Watch(...)`

## CURRENT STATUS

THIS PACKAGE DOES NOT WORK YET. IT'S A CONCEPT/STUDY.

* Wrote preliminary API design for fsnotify
* Seeing how I would write fsnotify-inotify backend

## SYNOPSIS

```go

import (
  "context"
  "os/signal"
  "syscall"

  "github.com/lestrrat-go/fsnotify"
)

func main() {
  ctx, cancel := signal.NotifyContext(
    context.Background(),
    syscall.SIGHUP,
    syscall.SIGTERM,
    syscall.SIGQUIT,
  )
  defer cancel()


  w := fsnotify.New()
  w.Add(`/path/fo/target`)

  evCh := make(chan *fsnotify.Event)
  errCh := make(chan error)
  go w.Watch(ctx, 
    fsnotify.WithEventSink(fsnotify.ChannelEventSink(evCh)),
    fsnotify.WithErrorSink(fsnotify.ChannelErrorSink(errCh)),
  )

  for {
    select {
    case <-ctx.Done():
      return
    case ev := <-evCh:
      ...
    case err := <-errCh:
      ...
    }
  }
}
```
