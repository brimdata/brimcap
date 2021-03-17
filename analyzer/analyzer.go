package analyzer

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"sync/atomic"

	"github.com/brimsec/brimcap/ztail"
	"github.com/brimsec/zq/compiler/ast"
	"github.com/brimsec/zq/driver"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
)

type Analyzer interface {
	zbuf.ReadCloser
	BytesRead() int64
}

type Config struct {
	Args       []string
	Cmd        string
	Globs      []string
	Launcher   Launcher
	ReaderOpts zio.ReaderOpts
	Shaper     ast.Proc
}

func (c Config) GetLauncher() (Launcher, error) {
	if c.Cmd != "" {
		return LauncherFromPath(c.Cmd, c.Args...)
	}
	return c.Launcher, nil
}

type analyzer struct {
	cancel   context.CancelFunc
	config   Config
	err      error
	logdir   string
	reader   *readCounter
	zreader  zbuf.Reader
	procDone chan struct{}
	tailer   *ztail.Tailer
	warn     chan string
	zctx     *resolver.Context
}

func New(zctx *resolver.Context, r io.Reader, conf Config) (Analyzer, error) {
	return NewWithContext(context.Background(), zctx, r, conf)
}

func NewWithContext(ctx context.Context, zctx *resolver.Context, r io.Reader, conf Config) (Analyzer, error) {
	ctx, cancel := context.WithCancel(ctx)
	a := &analyzer{
		cancel:   cancel,
		config:   conf,
		procDone: make(chan struct{}),
		reader:   &readCounter{reader: r},
		zctx:     zctx,
	}
	if err := a.run(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (p *analyzer) Read() (*zng.Record, error) {
	rec, err := p.zreader.Read()
	if rec == nil && err == nil {
		// If EOS received and done channel is closed, return the process error.
		// The tailer may have been closed because the process exited with an
		// error.
		select {
		case <-p.procDone:
			err = p.err
		default:
		}
	}
	return rec, err
}

func (p *analyzer) run(ctx context.Context) error {
	ln, err := p.config.GetLauncher()
	if err != nil {
		return err
	}

	logdir, err := ioutil.TempDir("", "zqd-pcap-ingest-")
	if err != nil {
		return err
	}

	waiter, err := ln(ctx, logdir, p.reader)
	if err != nil {
		os.RemoveAll(logdir)
		return err
	}

	tailer, err := ztail.New(p.zctx, logdir, p.config.ReaderOpts, p.config.Globs...)
	if err != nil {
		os.RemoveAll(logdir)
		return err
	}

	go func() {
		p.err = waiter.Wait()
		close(p.procDone)

		// Tell DirReader to stop tail files, which will in turn cause an EOF on
		// zbuf.Read stream when remaining data has been read.
		if err := p.tailer.Stop(); p.err == nil {
			p.err = err
		}
	}()

	p.zreader = tailer
	p.tailer = tailer
	p.logdir = logdir
	if p.config.Shaper != nil {
		p.zreader, err = driver.NewReader(ctx, p.config.Shaper, p.zctx, tailer)
		if err != nil {
			os.RemoveAll(logdir)
			return err
		}
	}

	return nil
}

func (p *analyzer) BytesRead() int64 {
	return p.reader.Bytes()
}

// Close shutdowns the current process (if it is still active), shutdowns the
// go-routine tailing for logs and removes the temporary log directory.
func (p *analyzer) Close() error {
	p.cancel()
	<-p.procDone

	err := p.tailer.Close()
	if err2 := os.RemoveAll(p.logdir); err == nil {
		err = err2
	}
	return err
}

type readCounter struct {
	reader io.Reader
	count  int64
}

func (r *readCounter) Read(b []byte) (int, error) {
	n, err := r.reader.Read(b)
	atomic.AddInt64(&r.count, int64(n))
	return n, err
}

func (r *readCounter) Bytes() int64 {
	return atomic.LoadInt64(&r.count)
}
