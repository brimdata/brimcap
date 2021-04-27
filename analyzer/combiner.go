package analyzer

import (
	"context"
	"io"

	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zson"
)

type combiner struct {
	analyzers []zio.Reader
	cancel    context.CancelFunc
	combiner  *zio.Combiner
	pipes     []*io.PipeWriter
}

func Combiner(zctx *zson.Context, pcap io.Reader, confs ...Config) Interface {
	return CombinerWithContext(context.Background(), zctx, pcap, confs...)
}

func CombinerWithContext(ctx context.Context, zctx *zson.Context, pcap io.Reader, confs ...Config) Interface {
	if len(confs) == 1 {
		return NewWithContext(ctx, zctx, pcap, confs[0])
	}

	ctx, cancel := context.WithCancel(ctx)

	pipes := make([]*io.PipeWriter, len(confs)-1)
	readers := make([]zio.Reader, len(confs))
	for i, conf := range confs {
		r := pcap
		// Do not pipe last analyzer. It will be responsible for pulling the
		// stream along.
		if i+1 < len(confs) {
			r, pipes[i] = io.Pipe()
			// Use a special variant io.TeeReader that ensures the pipe reader
			// receives errors from the parent reader. Needed because otherwise
			// some processes wouldn't receive and EOF and exit.
			pcap = &tee{pcap, pipes[i]}
		}

		readers[i] = NewWithContext(ctx, zctx, r, conf)
	}

	return &combiner{
		analyzers: readers,
		cancel:    cancel,
		combiner:  zio.NewCombiner(context.TODO(), readers),
		pipes:     pipes,
	}
}

func (p *combiner) WarningHandler(w zio.Warner) {
	for _, a := range p.analyzers {
		a.(Interface).WarningHandler(w)
	}
}

func (m *combiner) Read() (*zng.Record, error) {
	return m.combiner.Read()
}

func (p *combiner) RecordsRead() (count int64) {
	for _, a := range p.analyzers {
		count += a.(Interface).RecordsRead()
	}
	return
}

func (m *combiner) BytesRead() int64 {
	last := len(m.analyzers) - 1
	return m.analyzers[last].(Interface).BytesRead()
}

func (m *combiner) Close() error {
	for _, w := range m.pipes {
		w.Close()
	}
	defer m.cancel()
	return zio.CloseReaders(m.analyzers)
}

// tee is a version of io.TeeReader that takes an io.PipeWriter instead of a
// generic io.Writer and calls if CloseWithError on the writer should reader
// return an error.
type tee struct {
	r io.Reader
	w *io.PipeWriter
}

func (t *tee) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	if n > 0 {
		if n, err := t.w.Write(p[:n]); err != nil {
			return n, err
		}
	}
	if err != nil {
		t.w.CloseWithError(err)
	}
	return
}
