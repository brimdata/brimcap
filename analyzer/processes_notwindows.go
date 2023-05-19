//go:build !windows

package analyzer

import (
	"syscall"
)

var errPipe = syscall.EPIPE
