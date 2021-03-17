package analyzer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	mockanalyzer "github.com/brimsec/brimcap/analyzer/mock"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/golang/mock/gomock"
)

func TestAnalyzerErrorOnLaunch(t *testing.T) {
	expected := errors.New("could not find process")
	_, err := New(resolver.NewContext(), nil, Config{
		Launcher: func(_ context.Context, _ string, _ io.Reader) (ProcessWaiter, error) {
			return nil, expected
		},
	})

	if !errors.Is(err, expected) {
		t.Errorf("expected error to equal %v, got %v", expected, err)
	}
}

func TestAnalyzerErrorOnRead(t *testing.T) {
	expected := errors.New("process quit unexpectedly")

	r, err := New(resolver.NewContext(), nil, Config{
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
	if err != nil {
		t.Errorf("expected error to be nil, go %v", err)
	}

	if _, err = r.Read(); !errors.Is(err, expected) {
		t.Errorf("expected error to equal %v, got %v", expected, err)
	}
}

func TestAnalyzerRemovesLogDir(t *testing.T) {
	const expected = `{msg:"record1"}`
	dirpath := make(chan string, 1)

	r, err := New(resolver.NewContext(), nil, Config{
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
	if err != nil {
		t.Fatalf("expected error to be nil, got %v", err)
	}

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("expected error to be nil, got %v", err)

	} else if recstr := rec.Type.ZSONOf(rec.Bytes); recstr != expected {
		t.Fatalf("expected record to equal %q, got %q", recstr, expected)
	}

	if rec, err = r.Read(); rec != nil || err != nil {
		t.Errorf("expected EOS, got rec=%v, err=%v", rec, err)
	}

	if err = r.Close(); err != nil {
		t.Errorf("expected error to be nil, got %v", err)
	}

	if info, _ := os.Stat(<-dirpath); info != nil {
		t.Errorf("expected path to not exist, got %v", info)
	}
}

func TestAnalyzerCloseCancelsCtx(t *testing.T) {
	errChan := make(chan error, 1)
	r, err := New(resolver.NewContext(), nil, Config{
		ReaderOpts: zio.ReaderOpts{Format: "zson"},
		Launcher: func(ctx context.Context, dir string, _ io.Reader) (ProcessWaiter, error) {
			ctrl := gomock.NewController(t)
			waiter := mockanalyzer.NewMockProcessWaiter(ctrl)
			waiter.EXPECT().
				Wait().
				DoAndReturn(func() error {
					ctx.Done()
					errChan <- ctx.Err()
					return ctx.Err()
				})
			return waiter, nil
		},
	})
	if err != nil {
		t.Fatalf("expected error to be nil, got %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("expected error to be nil, got %v", err)
	}

	if err := <-errChan; !errors.Is(err, context.Canceled) {
		t.Errorf("expected error t be %v, got %v", context.Canceled, err)
	}
}
