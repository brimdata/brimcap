package analyzer

import (
	"context"
	"io"

	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
)

type multiAnalyzer struct {
	analyzers []zbuf.Reader
	cancel    context.CancelFunc
	combiner  *zbuf.Combiner
}

func Multi(zctx *resolver.Context, pcap io.Reader, confs ...Config) (Analyzer, error) {
	if len(confs) == 1 {
		return New(zctx, pcap, confs[0])
	}

	ctx, cancel := context.WithCancel(context.Background())

	readers := make([]zbuf.Reader, len(confs))
	for i, conf := range confs {
		r := pcap
		// Do not pipe last analyzer. It will be responsible for pulling the
		// stream along.
		if i+1 < len(confs) {
			var w *io.PipeWriter
			r, w = io.Pipe()
			// Use a special variant io.TeeReader that ensures the pipe reader
			// receives errors from the parent reader. Needed because otherwise
			// some processes wouldn't receive and EOF and exit.
			pcap = &tee{pcap, w}
		}

		var err error
		readers[i], err = NewWithContext(ctx, zctx, r, conf)
		if err != nil {
			cancel()
			return nil, err
		}
	}

	return &multiAnalyzer{
		analyzers: readers,
		cancel:    cancel,
		combiner:  zbuf.NewCombiner(context.TODO(), readers),
	}, nil
}

func (m *multiAnalyzer) Read() (*zng.Record, error) {
	rec, err := m.combiner.Read()
	if err != nil {
		// If an error is encountered, cancel the context so the other processes
		// shutdown.
		m.cancel()
	}
	return rec, err
}

func (m *multiAnalyzer) BytesRead() int64 {
	last := len(m.analyzers) - 1
	return m.analyzers[last].(Analyzer).BytesRead()
}

func (m *multiAnalyzer) Close() error {
	defer m.cancel()
	return zbuf.CloseReaders(m.analyzers)
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
