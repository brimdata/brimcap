package analyzer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/brimdata/zed/zio"
	"golang.org/x/sync/errgroup"
)

type operation struct {
	counter *writeCounter
	group   *errgroup.Group
}

func (o *operation) bytesRead() int64 { return atomic.LoadInt64(&o.counter.written) }
func (o *operation) wait() error      { return o.group.Wait() }

func runProcesses(ctx context.Context, r io.Reader, confs ...Config) (*operation, error) {
	var writers []io.Writer
	group, ctx := errgroup.WithContext(ctx)
	for _, conf := range confs {
		cmd, err := command(conf)
		if err != nil {
			return nil, err
		}
		group.Go(cmd.Run)
		writers = append(writers, cmd)
	}
	writeCounter := new(writeCounter)
	writers = append(writers, writeCounter)
	group.Go(func() error {
		_, err := io.Copy(io.MultiWriter(writers...), r)
		for _, w := range writers {
			if closer, ok := w.(io.Closer); ok {
				closer.Close()
			}
		}
		return err
	})
	return &operation{
		counter: writeCounter,
		group:   group,
	}, nil
}

func command(conf Config) (*wrappedCmd, error) {
	cmd := exec.Command(conf.Cmd, conf.Args...)
	cmd.Dir = conf.WorkDir
	pw, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	return &wrappedCmd{
		Cmd:         cmd,
		stderrPath:  conf.StderrPath,
		stderrSaver: &prefixSuffixSaver{N: 32 << 10},
		stdinWriter: pw,
		stdoutPath:  conf.StdoutPath,
		stdoutSaver: &prefixSuffixSaver{N: 32 << 10},
	}, nil
}

type wrappedCmd struct {
	*exec.Cmd
	stderrPath  string
	stderrSaver *prefixSuffixSaver
	stdinWriter io.WriteCloser
	stdoutPath  string
	stdoutSaver *prefixSuffixSaver
}

func (c *wrappedCmd) Write(b []byte) (int, error) {
	n, err := c.stdinWriter.Write(b)
	// Broken pipe errors and ErrClose are a result of a process shutting down.
	// Since this may be a case of the process legitimately exiting without
	// reading all data, we ignore these errors and pretend the write was
	// successful so as not to hold up data getting sent to other processes.
	if isPipe(err) || errors.Is(err, fs.ErrClosed) {
		return len(b), nil
	}
	return n, err
}

func (c *wrappedCmd) Close() error {
	return c.stdinWriter.Close()
}

func (c *wrappedCmd) Run() error {
	stderr, err := stdioWriter(c.stderrPath, c.stderrSaver)
	if err != nil {
		return err
	}
	defer stderr.Close()
	stdout, err := stdioWriter(c.stdoutPath, c.stdoutSaver)
	if err != nil {
		return err
	}
	defer stdout.Close()
	c.Cmd.Stderr, c.Cmd.Stdout = stderr, stdout
	return c.error(c.Cmd.Run())
}

func stdioWriter(path string, saver *prefixSuffixSaver) (io.WriteCloser, error) {
	if path == "" {
		return zio.NopCloser(saver), nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := io.MultiWriter(f, saver)
	return struct {
		io.Writer
		io.Closer
	}{w, f}, nil
}

func (c *wrappedCmd) error(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &ProcessExitError{
			Err:    exitErr,
			Path:   c.Cmd.Path,
			Stderr: c.stderrSaver.Bytes(),
			Stdout: c.stdoutSaver.Bytes(),
		}
	}
	if err != nil {
		name := filepath.Base(c.Cmd.Path)
		return fmt.Errorf("%s process error: %w", name, err)
	}
	return nil
}

type writeCounter struct {
	written int64
}

func (w *writeCounter) Write(b []byte) (int, error) {
	n := len(b)
	atomic.AddInt64(&w.written, int64(n))
	return n, nil
}

type ProcessExitError struct {
	Err    *exec.ExitError
	Path   string
	Stderr []byte
	Stdout []byte
}

func (p *ProcessExitError) Error() string {
	builder := new(strings.Builder)
	name := filepath.Base(p.Path)
	fmt.Fprintf(builder, "%s exited with code %d\n", name, p.Err.ExitCode())

	if p.Stdout != nil {
		fmt.Fprintln(builder, "stdout:")
		builder.Write(p.Stdout)
	} else {
		fmt.Fprintln(builder, "stdout: (no output)")
	}

	if p.Stderr != nil {
		fmt.Fprintln(builder, "stderr:")
		builder.Write(p.Stderr)
	} else {
		fmt.Fprintln(builder, "stderr: (no output)")
	}

	return builder.String()
}

// prefixSuffixSaver is an io.Writer which retains the first N bytes
// and the last N bytes written to it. The Bytes() methods reconstructs
// it with a pretty error message.
// Taken from github.com/golang/go/blob/master/src/os/exec/exec.go.
type prefixSuffixSaver struct {
	N         int // max size of prefix or suffix
	prefix    []byte
	suffix    []byte // ring buffer once len(suffix) == N
	suffixOff int    // offset to write into suffix
	skipped   int64
}

func (w *prefixSuffixSaver) Write(p []byte) (n int, err error) {
	lenp := len(p)
	p = w.fill(&w.prefix, p)

	// Only keep the last w.N bytes of suffix data.
	if overage := len(p) - w.N; overage > 0 {
		p = p[overage:]
		w.skipped += int64(overage)
	}
	p = w.fill(&w.suffix, p)

	// w.suffix is full now if p is non-empty. Overwrite it in a circle.
	for len(p) > 0 { // 0, 1, or 2 iterations.
		n := copy(w.suffix[w.suffixOff:], p)
		p = p[n:]
		w.skipped += int64(n)
		w.suffixOff += n
		if w.suffixOff == w.N {
			w.suffixOff = 0
		}
	}
	return lenp, nil
}

// fill appends up to len(p) bytes of p to *dst, such that *dst does not
// grow larger than w.N. It returns the un-appended suffix of p.
func (w *prefixSuffixSaver) fill(dst *[]byte, p []byte) (pRemain []byte) {
	if remain := w.N - len(*dst); remain > 0 {
		add := len(p)
		if remain < add {
			add = remain
		}
		*dst = append(*dst, p[:add]...)
		p = p[add:]
	}
	return p
}

func (w *prefixSuffixSaver) Bytes() []byte {
	if w.suffix == nil {
		return w.prefix
	}
	if w.skipped == 0 {
		return append(w.prefix, w.suffix...)
	}
	var buf bytes.Buffer
	buf.Grow(len(w.prefix) + len(w.suffix) + 50)
	buf.Write(w.prefix)
	buf.WriteString("\n... omitting ")
	buf.WriteString(strconv.FormatInt(w.skipped, 10))
	buf.WriteString(" bytes ...\n")
	buf.Write(w.suffix[w.suffixOff:])
	buf.Write(w.suffix[:w.suffixOff])
	return buf.Bytes()
}
