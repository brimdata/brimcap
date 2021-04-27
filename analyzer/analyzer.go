package analyzer

import (
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/brimdata/brimcap/ztail"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/driver"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zson"
)

type Interface interface {
	zio.ReadCloser
	BytesRead() int64
	RecordsRead() int64
	WarningHandler(zio.Warner)
}

type analyzer struct {
	cancel   context.CancelFunc
	config   Config
	ctx      context.Context
	once     sync.Once
	procErr  error
	procDone chan struct{}
	reader   *readCounter
	records  int64
	tailer   *ztail.Tailer
	warner   zio.Warner
	workdir  string
	zctx     *zson.Context
	zreader  zio.Reader
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

func (p *analyzer) WarningHandler(w zio.Warner) {
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

func (p *analyzer) run() (err error) {
	var shaper ast.Proc
	if p.config.Shaper != "" {
		if shaper, err = compiler.ParseProc(p.config.Shaper); err != nil {
			close(p.procDone)
			return err
		}
	}

	p.workdir = p.config.WorkDir
	if p.workdir == "" {
		if p.workdir, err = os.MkdirTemp("", "brimcap-"); err != nil {
			close(p.procDone)
			return err
		}
	}

	process, err := newLauncher(p.config)(p.ctx, p.workdir, p.reader)
	if err != nil {
		close(p.procDone)
		p.removeWorkDir()
		return err
	}

	tailer, err := ztail.New(p.zctx, p.workdir, p.config.ReaderOpts, p.config.Globs...)
	if err != nil {
		close(p.procDone)
		p.removeWorkDir()
		return err
	}
	tailer.WarningHandler(p.warner)

	go func() {
		p.procErr = process.Wait()
		close(p.procDone)
		// Tell DirReader to stop tail files, which will in turn cause an EOF on
		// zbuf.Read stream when remaining data has been read.
		if err := p.tailer.Stop(); p.procErr == nil {
			p.procErr = err
		}

	}()

	p.zreader = tailer
	p.tailer = tailer
	if shaper != nil {
		p.zreader, err = driver.NewReader(p.ctx, shaper, p.zctx, p.zreader)
		if err != nil {
			close(p.procDone)
			tailer.Close()
			p.removeWorkDir()
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

func (p *analyzer) removeWorkDir() error {
	if p.config.WorkDir == "" {
		return os.RemoveAll(p.workdir)
	}
	return nil
}

// Close shutdowns the current process (if it is still active), shutdowns the
// go-routine tailing for logs and removes the temporary log directory.
func (p *analyzer) Close() (err error) {
	p.cancel()
	<-p.procDone

	if p.tailer != nil {
		err = p.tailer.Close()
	}

	if err2 := p.removeWorkDir(); err == nil {
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
