package pcapio

import (
	"fmt"
)

type Warner interface {
	Warn(msg string) error
}

type ErrInvalidPcap struct {
	err error
}

func NewErrInvalidPcap(err error) error {
	return &ErrInvalidPcap{err: err}
}

func errInvalidf(format string, a ...interface{}) error {
	return NewErrInvalidPcap(fmt.Errorf(format, a...))
}

func (e *ErrInvalidPcap) Is(target error) bool {
	_, ok := target.(*ErrInvalidPcap)
	return ok
}

func (e *ErrInvalidPcap) Unwrap() error {
	return e.err
}

func (e *ErrInvalidPcap) Error() string {
	return "invalid pcap: " + e.err.Error()
}
