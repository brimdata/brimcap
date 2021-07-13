package analyzer

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/brimdata/brimcap/ztail"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/driver"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zson"
	"go.uber.org/multierr"
)

type reader struct {
	reader  zio.Reader
	records int64
	tailers tailers
}

func newReader(ctx context.Context, warner zio.Warner, confs ...Config) (*reader, error) {
	var tailers tailers
	var readers []zio.Reader
	zctx := zson.NewContext()
	for _, conf := range confs {
		reader, tailer, err := tailOne(ctx, zctx, conf, warner)
		if err != nil {
			tailers.close()
			return nil, err
		}
		tailers = append(tailers, tailer)
		readers = append(readers, reader)
	}
	return &reader{
		reader:  zio.NewCombiner(ctx, readers),
		tailers: tailers,
	}, nil
}

func (h *reader) Read() (*zng.Record, error) {
	rec, err := h.reader.Read()
	if rec != nil {
		atomic.AddInt64(&h.records, 1)
	}
	return rec, err
}

func (h *reader) stop() error        { return h.tailers.stop() }
func (h *reader) close() (err error) { return h.tailers.close() }

func tailOne(ctx context.Context, zctx *zson.Context, conf Config, warner zio.Warner) (zio.Reader, *ztail.Tailer, error) {
	var shaper ast.Proc
	if conf.Shaper != "" {
		var err error
		if shaper, err = compiler.ParseProc(conf.Shaper); err != nil {
			return nil, nil, err
		}
	}
	wrapped := wrappedReader{warner: warner, cmd: conf.Cmd}
	tailer, err := ztail.New(zctx, conf.WorkDir, conf.ReaderOpts, wrapped, conf.Globs...)
	if err != nil {
		return nil, nil, err
	}
	wrapped.reader = tailer
	if shaper != nil {
		wrapped.reader, err = driver.NewReader(ctx, shaper, zctx, tailer)
		if err != nil {
			tailer.Close()
			return nil, nil, err
		}
	}
	return wrapped, tailer, nil
}

type wrappedReader struct {
	cmd    string
	warner zio.Warner
	reader zio.Reader
}

func (w wrappedReader) Warn(msg string) error {
	return w.warner.Warn(fmt.Sprintf("%s: %s", w.cmd, msg))
}

func (w wrappedReader) Read() (*zng.Record, error) {
	rec, err := w.reader.Read()
	if err != nil {
		err = fmt.Errorf("%s: %w", w.cmd, err)
	}
	return rec, err
}

type tailers []*ztail.Tailer

func (t tailers) stop() error {
	var merr error
	for _, tailer := range t {
		if err := tailer.Stop(); err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	return merr
}

func (t tailers) close() error {
	var merr error
	for _, tailer := range t {
		if err := tailer.Close(); err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	return merr
}
