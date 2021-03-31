// Package ztail provides facilities for watching a directory of logs, tailing
// all the files created within it and transforming the data into zng data.
package ztail

import (
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/brimdata/brimcap/tail"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/detector"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zng/resolver"
)

// Tailer is a zbuf.Reader that watches a specified directory and starts
// tailing existing and newly created files in the directory for new logs. Newly
// written log data are transformed into *zng.Records and returned on a
// first-come-first serve basis.
type Tailer struct {
	forceClose uint32
	opts       zio.ReaderOpts
	readers    map[string]*tail.File
	tailer     *tail.Dir
	warner     zbuf.Warner
	zctx       *resolver.Context

	// synchronization primitives
	results chan result
	once    sync.Once
	wg      sync.WaitGroup
}

func New(zctx *resolver.Context, dir string, opts zio.ReaderOpts, globs ...string) (*Tailer, error) {
	dir = filepath.Clean(dir)
	tailer, err := tail.TailDir(dir, globs...)
	if err != nil {
		return nil, err
	}
	r := &Tailer{
		opts:    opts,
		readers: make(map[string]*tail.File),
		results: make(chan result, 5),
		tailer:  tailer,
		zctx:    zctx,
	}
	return r, nil
}

type result struct {
	rec *zng.Record
	err error
}

func (d *Tailer) start() {
	var err error
	for {
		ev, ok := <-d.tailer.Events
		// Watcher closed. Enstruct all go routines to stop tailing files so
		// they read remaining data then exit.
		if !ok {
			forceClose := atomic.LoadUint32(&d.forceClose) == 1
			d.stopReaders(forceClose)
			break
		}
		if ev.Err != nil {
			err = ev.Err
			d.stopReaders(true)
			break
		}
		if ev.Op.Exists() {
			if terr := d.tailFile(ev.Name); terr != nil {
				err = terr
				d.stopReaders(true)
				break
			}
		}
	}
	// Wait for all tail go routines to stop. We are about to close the results
	// channel and do not want a write to closed channel panic.
	d.wg.Wait()
	// signfy EOS and close channel
	d.results <- result{err: err}
	close(d.results)
}

// stopReaders instructs all open TFile to stop tailing their respective files.
// If close is set to false, the readers will read through the remaining data
// in their files before emitting EOF. If close is set to true, the file
// descriptors will be closed and no further data will be read.
func (d *Tailer) stopReaders(close bool) {
	for _, r := range d.readers {
		if close {
			r.Close()
		}
		r.Stop()
	}
}

func (d *Tailer) tailFile(file string) error {
	if _, ok := d.readers[file]; ok {
		return nil
	}
	f, err := tail.TailFile(file)
	if err == tail.ErrIsDir {
		return nil
	}
	if err != nil {
		return err
	}
	d.readers[file] = f
	d.wg.Add(1)
	go func() {
		var zr zbuf.Reader
		zr, err = detector.OpenFromNamedReadCloser(d.zctx, f, file, d.opts)
		if err != nil {
			d.results <- result{err: err}
			return
		}
		if d.warner != nil {
			zr = zbuf.NewWarningReader(zr, d.warner)
		}
		var res result
		for {
			res.rec, res.err = zr.Read()
			if res.rec != nil || res.err != nil {
				d.results <- res
			}
			if res.rec == nil || res.err != nil {
				d.wg.Done()
				return
			}
		}
	}()
	return nil
}

func (d *Tailer) WarningHandler(warner zbuf.Warner) {
	d.warner = warner
}

func (d *Tailer) Read() (*zng.Record, error) {
	d.once.Do(func() { go d.start() })
	res, ok := <-d.results
	if !ok {
		// already closed return EOS
		return nil, nil
	}
	if res.err != nil {
		d.tailer.Stop() // exits loop
		// drain results
		for range d.results {
		}
	}
	return res.rec, res.err
}

// Stop instructs the directory watcher and indiviual file watchers to stop
// watching for changes. Read() will emit EOS when the remaining unread data
// in files has been read.
func (d *Tailer) Stop() error {
	return d.tailer.Stop()
}

func (d *Tailer) Close() error {
	atomic.StoreUint32(&d.forceClose, 1)
	return d.tailer.Stop()
}
