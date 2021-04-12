package analyzer

import (
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/brimdata/brimcap/ztail"
	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/driver"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zson"
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
	zctx     *zson.Context
	zreader  zbuf.Reader
}

func New(zctx *zson.Context, r io.Reader, conf Config) Interface {
	return NewWithContext(context.Background(), zctx, r, conf)
}

func NewWithContext(ctx context.Context, zctx *zson.Context, r io.Reader, conf Config) Interface {
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
		close(p.procDone)
		return err
	}

	logdir, err := os.MkdirTemp("", "brimcap-")
	if err != nil {
		close(p.procDone)
		return err
	}

	waiter, err := ln(p.ctx, logdir, p.reader)
	if err != nil {
		close(p.procDone)
		os.RemoveAll(logdir)
		return err
	}

	tailer, err := ztail.New(p.zctx, logdir, p.config.ReaderOpts, p.config.Globs...)
	if err != nil {
		close(p.procDone)
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
			close(p.procDone)
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
func (p *analyzer) Close() (err error) {
	p.cancel()
	<-p.procDone

	if p.tailer != nil {
		err = p.tailer.Close()
	}
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
