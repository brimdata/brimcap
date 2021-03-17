package analyzer

import (
	"context"
	"io"
	"os/exec"
)

// Launcher is a function to start a pcap import process given a context,
// input pcap reader, and target output dir. If the process is started
// successfully, a ProcessWaiter and nil error are returned. If there
// is an error starting the Process, that error is returned.
type Launcher func(context.Context, string, io.Reader) (ProcessWaiter, error)

// LauncherFromPath returns a Launcher instance that will execute a pcap
// to zeek log transformation, using the provided path to the command.
// zeekpath should point to an executable or script that:
// - expects to receive a pcap file on stdin
// - writes the resulting logs into its working directory
func LauncherFromPath(path string, args ...string) (Launcher, error) {
	return func(ctx context.Context, dir string, r io.Reader) (ProcessWaiter, error) {
		cmd := exec.CommandContext(ctx, path, args...)
		cmd.Stdin = r
		cmd.Dir = dir
		p := NewProcess(cmd)
		return p, p.Start()
	}, nil
}
