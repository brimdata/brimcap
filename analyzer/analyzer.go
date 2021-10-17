package analyzer

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"github.com/brimdata/zed/zio"
	"golang.org/x/sync/errgroup"
)

type Stats struct {
	BytesRead     int64
	ValuesWritten int64
}

type Display interface {
	zio.Warner
	Stats(Stats) error
}

// Run executes the provided configs against the pcap stream, writing the
// produced values to w. If interval is > 0, the d.Stats will be called
// at that interval.
func Run(ctx context.Context, pcap io.Reader, w zio.Writer, d Display, interval time.Duration, cs ...Config) error {
	confs := Configs(cs).removeDisabled()
	if err := confs.Validate(); err != nil {
		return err
	}
	cleanup, err := confs.ensureWorkDirs()
	if err != nil {
		return err
	}
	defer cleanup()
	group, ctx := errgroup.WithContext(ctx)
	r, err := newReader(ctx, d, confs...)
	if err != nil {
		return err
	}
	procs, err := runProcesses(ctx, pcap, confs...)
	if err != nil {
		r.close()
		return err
	}
	var valueCount int64
	group.Go(func() error {
		defer r.stop()
		return procs.wait()
	})
	group.Go(func() error {
		for ctx.Err() == nil {
			zv, err := r.Read()
			if zv == nil || err != nil {
				return err
			}
			if err := w.Write(zv); err != nil {
				return err
			}
			atomic.AddInt64(&valueCount, 1)
		}
		return nil
	})
	if interval > 0 {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					d.Stats(Stats{
						BytesRead:     procs.bytesRead(),
						ValuesWritten: atomic.LoadInt64(&valueCount),
					})
				}
			}
		}()
	}
	err = group.Wait()
	if err == nil {
		// Send final Stats upon completion.
		d.Stats(Stats{
			BytesRead:     procs.bytesRead(),
			ValuesWritten: atomic.LoadInt64(&valueCount),
		})
	}
	return err
}
