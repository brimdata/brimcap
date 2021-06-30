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

type Driver interface {
	zio.Warner
	Stats(Stats) error
}

// Run executes the provided configs against the pcap stream, writing the
// produced records to w. If interval is > 0, d.Stats is called
// at that interval.
func Run(ctx context.Context, pcap io.Reader, w zio.Writer, d Driver, interval time.Duration, confs ...Config) error {
	if err := Configs(confs).Validate(); err != nil {
		return err
	}
	group, ctx := errgroup.WithContext(ctx)
	r, err := NewReader(ctx, d, confs...)
	if err != nil {
		return err
	}
	procs, err := RunProcesses(ctx, pcap, confs...)
	if err != nil {
		r.Close()
		return err
	}
	var recordCount int64
	group.Go(func() error {
		defer r.Stop()
		return procs.Wait()
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
						BytesRead:      procs.BytesRead(),
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
			BytesRead:      procs.BytesRead(),
			RecordsWritten: atomic.LoadInt64(&recordCount),
		})
	}
	return err
}
