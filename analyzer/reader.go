package analyzer

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/brimdata/brimcap/ztail"
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/runtime"
	"github.com/brimdata/zed/zio"
	"go.uber.org/multierr"
)

type reader struct {
	reader  zio.Reader
	tailers tailers
	values  int64
}

func newReader(ctx context.Context, warner ztail.Warner, confs ...Config) (*reader, error) {
	var tailers tailers
	var readers []zio.Reader
	zctx := zed.NewContext()
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

func (h *reader) Read() (*zed.Value, error) {
	zv, err := h.reader.Read()
	if zv != nil {
		atomic.AddInt64(&h.values, 1)
	}
	return zv, err
}

func (h *reader) stop() error        { return h.tailers.stop() }
func (h *reader) close() (err error) { return h.tailers.close() }

func tailOne(ctx context.Context, zctx *zed.Context, conf Config, warner ztail.Warner) (zio.Reader, *ztail.Tailer, error) {
	var shaper ast.Seq
	if conf.Shaper != "" {
		var err error
		if shaper, err = compiler.Parse(conf.Shaper); err != nil {
			return nil, nil, err
		}
	}
	wrapped := wrappedReader{cmd: conf.Cmd, warner: warner}
	tailer, err := ztail.New(zctx, conf.WorkDir, conf.ReaderOpts, wrapped, conf.Globs...)
	if err != nil {
		return nil, nil, err
	}
	wrapped.reader = tailer
	if shaper != nil {
		query, err := runtime.CompileQuery(ctx, zctx, compiler.NewCompiler(), shaper, []zio.Reader{tailer})
		if err != nil {
			tailer.Close()
			return nil, nil, err
		}
		wrapped.reader = query.AsReader()
	}
	return wrapped, tailer, nil
}

type wrappedReader struct {
	cmd    string
	warner ztail.Warner
	reader zio.Reader
}

func (w wrappedReader) Warn(msg string) error {
	return w.warner.Warn(fmt.Sprintf("%s: %s", w.cmd, msg))
}

func (w wrappedReader) Read() (*zed.Value, error) {
	zv, err := w.reader.Read()
	if err != nil {
		err = fmt.Errorf("%s: %w", w.cmd, err)
	}
	return zv, err
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
