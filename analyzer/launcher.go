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

func newLauncher(conf Config) Launcher {
	if conf.Launcher != nil {
		return conf.Launcher
	}
	return func(ctx context.Context, dir string, r io.Reader) (ProcessWaiter, error) {
		cmd := exec.CommandContext(ctx, conf.Cmd, conf.Args...)
		cmd.Stdin = r
		cmd.Dir = dir
		p := NewProcess(cmd)
		if err := p.SetStdio(conf.StderrPath, conf.StdoutPath); err != nil {
			return nil, err
		}
		return p, p.Start()
	}
}
