package analyzer

import (
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/brimsec/brimcap/ztail"
	"github.com/brimsec/zq/compiler/ast"
	"github.com/brimsec/zq/driver"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
)

type Interface interface {
	zbuf.ReadCloser
	BytesRead() int64
	RecordsRead() int64
	WarningHandler(zbuf.Warner)
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
	ctx      context.Context
	logdir   string
	once     sync.Once
	procErr  error
	procDone chan struct{}
	reader   *readCounter
	records  int64
	tailer   *ztail.Tailer
	warner   zbuf.Warner
	zctx     *resolver.Context
	zreader  zbuf.Reader
}

func New(zctx *resolver.Context, r io.Reader, conf Config) Interface {
	return NewWithContext(context.Background(), zctx, r, conf)
}

func NewWithContext(ctx context.Context, zctx *resolver.Context, r io.Reader, conf Config) Interface {
	ctx, cancel := context.WithCancel(ctx)
	a := &analyzer{
		cancel:   cancel,
		config:   conf,
		ctx:      ctx,
		procDone: make(chan struct{}),
		reader:   &readCounter{reader: r},
		zctx:     zctx,
	}
	return a
}

func (p *analyzer) WarningHandler(w zbuf.Warner) {
	p.warner = w
}

func (p *analyzer) Read() (*zng.Record, error) {
	var err error
	p.once.Do(func() {
		err = p.run()
	})
	if err != nil {
		return nil, err
	}
	rec, err := p.zreader.Read()
	if rec == nil && err == nil {
		// If EOS received and done channel is closed, return the process error.
		// The tailer may have been closed because the process exited with an
		// error.
		select {
		case <-p.procDone:
			err = p.procErr
		default:
		}
	}
	if rec != nil {
		atomic.AddInt64(&p.records, 1)
	}
	return rec, err
}

func (p *analyzer) run() error {
	ln, err := p.config.GetLauncher()
	if err != nil {
		return err
	}

	logdir, err := os.MkdirTemp("", "brimcap-")
	if err != nil {
		return err
	}

	waiter, err := ln(p.ctx, logdir, p.reader)
	if err != nil {
		os.RemoveAll(logdir)
		return err
	}

	tailer, err := ztail.New(p.zctx, logdir, p.config.ReaderOpts, p.config.Globs...)
	if err != nil {
		os.RemoveAll(logdir)
		return err
	}
	tailer.WarningHandler(p.warner)

	go func() {
		p.procErr = waiter.Wait()
		close(p.procDone)

		// Tell DirReader to stop tail files, which will in turn cause an EOF on
		// zbuf.Read stream when remaining data has been read.
		if err := p.tailer.Stop(); p.procErr == nil {
			p.procErr = err
		}
	}()

	p.zreader = tailer
	p.tailer = tailer
	p.logdir = logdir
	if p.config.Shaper != nil {
		p.zreader, err = driver.NewReader(p.ctx, p.config.Shaper, p.zctx, p.zreader)
		if err != nil {
			tailer.Close()
			os.RemoveAll(logdir)
			return err
		}
	}

	return nil
}

func (p *analyzer) RecordsRead() int64 {
	return atomic.LoadInt64(&p.records)
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
