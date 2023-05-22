//go:build !windows

package analyzer

import (
	"errors"
	"syscall"
)

func isPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
