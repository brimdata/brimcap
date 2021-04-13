package analyzer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	mockanalyzer "github.com/brimdata/brimcap/analyzer/mock"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zson"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzerErrorOnLaunch(t *testing.T) {
	expected := errors.New("could not find process")
	analyzer := New(zson.NewContext(), nil, Config{
		Launcher: func(_ context.Context, _ string, _ io.Reader) (ProcessWaiter, error) {
			return nil, expected
		},
	})

	_, err := analyzer.Read()
	assert.ErrorIs(t, err, expected)
}

func TestAnalyzerErrorOnRead(t *testing.T) {
	expected := errors.New("process quit unexpectedly")

	r := New(zson.NewContext(), nil, Config{
		Launcher: func(_ context.Context, _ string, _ io.Reader) (ProcessWaiter, error) {
			ctrl := gomock.NewController(t)
			waiter := mockanalyzer.NewMockProcessWaiter(ctrl)
			waiter.EXPECT().
				Wait().
				DoAndReturn(func() error {
					return expected
				})
			return waiter, nil
		},
	})

	if _, err := r.Read(); !errors.Is(err, expected) {
		t.Errorf("expected error to equal %v, got %v", expected, err)
	}
}

func TestAnalyzerRemovesLogDir(t *testing.T) {
	const expected = `{msg:"record1"}`
	dirpath := make(chan string, 1)

	r := New(zson.NewContext(), nil, Config{
		ReaderOpts: zio.ReaderOpts{Format: "zson"},
		Launcher: func(_ context.Context, dir string, _ io.Reader) (ProcessWaiter, error) {
			dirpath <- dir
			err := os.WriteFile(filepath.Join(dir, "test.log"), []byte(expected), 0600)
			if err != nil {
				return nil, err
			}

			ctrl := gomock.NewController(t)
			waiter := mockanalyzer.NewMockProcessWaiter(ctrl)
			waiter.EXPECT().
				Wait().
				DoAndReturn(func() error {
					return nil
				})
			return waiter, nil
		},
	})

	if rec, err := r.Read(); err != nil {
		t.Fatalf("expected error to be nil, got %v", err)
	} else if recstr := rec.Type.ZSONOf(rec.Bytes); recstr != expected {
		t.Fatalf("expected record to equal %q, got %q", recstr, expected)
	}

	if rec, err := r.Read(); rec != nil || err != nil {
		t.Errorf("expected EOS, got rec=%v, err=%v", rec, err)
	}

	if err := r.Close(); err != nil {
		t.Errorf("expected error to be nil, got %v", err)
	}

	if info, _ := os.Stat(<-dirpath); info != nil {
		t.Error("expected path to not exist but it does")
	}
}

func TestAnalyzerCloseCancelsCtx(t *testing.T) {
	// Some random records because otherwise the ndjson reader will not emit.
	var records = []byte(`
{"msg": "record1"}
{"msg": "record2"}
{"msg": "record3"}
{"msg": "record4"}`)
	errChan := make(chan error, 1)
	r := New(zson.NewContext(), nil, Config{
		ReaderOpts: zio.ReaderOpts{Format: "ndjson"},
		Launcher: func(ctx context.Context, dir string, _ io.Reader) (ProcessWaiter, error) {
			ctrl := gomock.NewController(t)
			waiter := mockanalyzer.NewMockProcessWaiter(ctrl)
			err := os.WriteFile(filepath.Join(dir, "test.log"), records, 0600)
			if err != nil {
				return nil, err
			}
			waiter.EXPECT().
				Wait().
				DoAndReturn(func() error {
					<-ctx.Done()
					errChan <- ctx.Err()
					return ctx.Err()
				})
			return waiter, nil
		},
	})

	// Call read to launch process.
	r.Read()
	if err := r.Close(); err != nil {
		t.Fatalf("expected error to be nil, got %v", err)
	}

	if err := <-errChan; !errors.Is(err, context.Canceled) {
		t.Errorf("expected error t be %v, got %v", context.Canceled, err)
	}
}
