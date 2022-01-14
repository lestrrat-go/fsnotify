package fsnotify

type ChannelEventSink chan *Event

func (sink ChannelEventSink) Event(ev *Event) {
	sink <- ev
}

type ChannelErrorSink chan error

func (sink ChannelErrorSink) Error(err error) {
	sink <- err
}
