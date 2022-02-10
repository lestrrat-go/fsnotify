package fsnotify

import "github.com/lestrrat-go/fsnotify/api"

type ChannelEventSink chan api.Event

func (sink ChannelEventSink) Event(ev api.Event) {
	sink <- ev
}

type ChannelErrorSink chan error

func (sink ChannelErrorSink) Error(err error) {
	sink <- err
}
