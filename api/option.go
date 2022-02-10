package api

import "github.com/lestrrat-go/option"

type Option = option.Interface
type identAck struct{}

type CommandOption interface {
	Option
	commandOption()
}

type commandOption struct {
	Option
}

func (*commandOption) commandOption() {}

func IsAck(ident interface{}) bool {
	return ident == identAck{}
}

func WithAck(b bool) CommandOption {
	return &commandOption{option.New(identAck{}, b)}
}
