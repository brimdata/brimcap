package analyzer

import (
	"errors"
	"syscall"
)

func isPipe(err error) bool {
	const ERROR_NO_DATA = syscall.Errno(232)
	return errors.Is(err, syscall.ERROR_BROKEN_PIPE) || errors.Is(err, ERROR_NO_DATA)
}
