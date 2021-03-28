package analyzer

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	mockanalyzer "github.com/brimsec/brimcap/analyzer/mock"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/golang/mock/gomock"
)

// test to ensure that an EOF on pcap read stream, eventually leads to an EOS.

func TestMultiAnalyzerEOS(t *testing.T) {
	ctrl := gomock.NewController(t)
	analyzer1 := Config{
		Launcher: func(_ context.Context, path string, r io.Reader) (ProcessWaiter, error) {
			waiter := mockanalyzer.NewMockProcessWaiter(ctrl)
			waiter.EXPECT().
				Wait().
				DoAndReturn(func() error {
					_, err := io.ReadAll(r)
					return err
				})
			return waiter, nil
		},
	}
	analyzer2 := Config{
		Launcher: func(ctx context.Context, path string, r io.Reader) (ProcessWaiter, error) {
			waiter := mockanalyzer.NewMockProcessWaiter(ctrl)
			waiter.EXPECT().
				Wait().
				DoAndReturn(func() error {
					_, err := io.ReadAll(r)
					return err
				})
			return waiter, nil
		},
	}

	r := strings.NewReader("some test data")
	reader := Multi(resolver.NewContext(), r, analyzer1, analyzer2)
	defer reader.Close()

	if rec, err := reader.Read(); rec != nil || err != nil {
		t.Errorf("expected EOS, got rec=%v, err=%v", rec, err)
	}
}

func TestMultiAnalyzerError(t *testing.T) {
	expected := errors.New("analyzer1 error")

	errCh := make(chan error, 1)
	ctrl := gomock.NewController(t)
	analyzer1 := Config{
		Launcher: func(_ context.Context, path string, r io.Reader) (ProcessWaiter, error) {
			waiter := mockanalyzer.NewMockProcessWaiter(ctrl)
			waiter.EXPECT().
				Wait().
				DoAndReturn(func() error {
					return <-errCh
				})
			return waiter, nil
		},
	}
	analyzer2 := Config{
		Launcher: func(ctx context.Context, path string, r io.Reader) (ProcessWaiter, error) {
			waiter := mockanalyzer.NewMockProcessWaiter(ctrl)
			waiter.EXPECT().
				Wait().
				DoAndReturn(func() error {
					<-ctx.Done()
					return ctx.Err()
				})
			return waiter, nil
		},
	}

	pr, _ := io.Pipe()
	reader := Multi(resolver.NewContext(), pr, analyzer1, analyzer2)

	errCh <- expected
	if _, err := reader.Read(); !errors.Is(err, expected) {
		t.Errorf("expected error to equal %v, got %v", expected, err)
	}

	if err := reader.Close(); err != nil {
		t.Errorf("expected error to be nil, got %v", err)
	}
}
