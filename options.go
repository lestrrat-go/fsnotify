package fsnotify

type Option = option.Interrface
type WatchOption interface {
	Option
	watchOption()
}

type watchOption struct {
	Option
}

func (*watchOption) watchOption() {}

type identErrorSink struct{}

func WithErrorSink(sink ErrorSink) WatchOption {
	return &watchOption{option.New(identErrorSink{}, sink)}
}
