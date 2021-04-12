//go:generate mockgen -destination=./mock/mock_process.go -package=mock github.com/brimdata/brimcap/analyzer ProcessWaiter

package analyzer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ProcessWaiter is an interface for interacting with a running process.
type ProcessWaiter interface {
	// Wait waits for a running process to exit, returning any errors that
	// occur.
	Wait() error
}

type Process struct {
	cmd     *exec.Cmd
	stderr  *prefixSuffixSaver
	stdout  *prefixSuffixSaver
	closers []io.Closer
}

func NewProcess(cmd *exec.Cmd) *Process {
	return &Process{
		cmd:    cmd,
		stderr: &prefixSuffixSaver{N: 32 << 10},
		stdout: &prefixSuffixSaver{N: 32 << 10},
	}
}

func (p *Process) SetStdio(stderr string, stdout string) error {
	if stderr != "" {
		f, err := os.Create(stderr)
		if err != nil {
			return p.error(err)
		}
		p.cmd.Stderr = f
		p.closers = append(p.closers, f)
	}

	if stdout != "" {
		f, err := os.Create(stdout)
		if err != nil {
			return p.error(err)
		}
		p.cmd.Stdout = f
		p.closers = append(p.closers, f)
	}

	return nil
}

func (p *Process) Start() error {
	if p.cmd.Stderr == nil {
		p.cmd.Stderr = p.stderr
	} else {
		p.cmd.Stderr = io.MultiWriter(p.stderr, p.cmd.Stderr)
	}

	if p.cmd.Stdout == nil {
		p.cmd.Stdout = p.stdout
	} else {
		p.cmd.Stdout = io.MultiWriter(p.cmd.Stdout, p.stdout)
	}

	err := p.cmd.Start()
	return p.error(err)
}

func (p *Process) Wait() error {
	err := p.cmd.Wait()
	for _, closer := range p.closers {
		closer.Close()
	}
	return p.error(err)
}

func (p *Process) error(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &ProcessExitError{
			Args:   p.cmd.Args,
			Err:    exitErr,
			Path:   p.cmd.Path,
			Stderr: p.stderr.Bytes(),
			Stdout: p.stdout.Bytes(),
		}
	}
	if err != nil {
		name := filepath.Base(p.cmd.Path)
		return fmt.Errorf("%s process error: %w", name, err)
	}

	return nil
}

type ProcessExitError struct {
	Args   []string
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
// Taken from github.com/golang/go/src/os/exec/exec.go
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
