package fsnotify

import (
	"github.com/lestrrat-go/fsnotify/api"
	"github.com/lestrrat-go/option"
)

type Option = option.Interface
type WatchOption interface {
	Option
	watchOption()
}

type watchOption struct {
	Option
}

func (*watchOption) watchOption() {}

type identErrorSink struct{}
type identEventSink struct{}

func WithErrorSink(sink api.ErrorSink) WatchOption {
	return &watchOption{option.New(identErrorSink{}, sink)}
}

func WithEventSink(sink api.EventSink) WatchOption {
	return &watchOption{option.New(identEventSink{}, sink)}
}
