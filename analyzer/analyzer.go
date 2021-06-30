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
	BytesRead      int64
	RecordsWritten int64
}

type Display interface {
	zio.Warner
	Stats(Stats) error
}

// Run executes the provided configs against the pcap stream, writing the
// produced records to w. If interval is > 0, the d.Stats will be called
// at that interval.
func Run(ctx context.Context, pcap io.Reader, w zio.Writer, d Display, interval time.Duration, confs ...Config) error {
	if err := Configs(confs).Validate(); err != nil {
		return err
	}
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
	var recordCount int64
	group.Go(func() error {
		defer r.stop()
		return procs.wait()
	})
	group.Go(func() error {
		for ctx.Err() == nil {
			rec, err := r.Read()
			if rec == nil || err != nil {
				return err
			}
			if err := w.Write(rec); err != nil {
				return err
			}
			atomic.AddInt64(&recordCount, 1)
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
						BytesRead:      procs.bytesRead(),
						RecordsWritten: atomic.LoadInt64(&recordCount),
					})
				}
			}
		}()
	}
	err = group.Wait()
	if err == nil {
		// Send final Stats upon completion.
		d.Stats(Stats{
			BytesRead:      procs.bytesRead(),
			RecordsWritten: atomic.LoadInt64(&recordCount),
		})
	}
	return err
}
