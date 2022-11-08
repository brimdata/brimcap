// Package ztail provides facilities for watching a directory of logs, tailing
// all the files created within it and transforming the data into zng data.
package ztail

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/brimdata/brimcap/tail"
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/anyio"
)

// Tailer is a zio.Reader that watches a specified directory and starts
// tailing existing and newly created files in the directory for new logs. Newly
// written log data are transformed into *zed.Values and returned on a
// first-come-first serve basis.
type Tailer struct {
	forceClose uint32
	opts       anyio.ReaderOpts
	readers    map[string]*tail.File
	tailer     *tail.Dir
	warner     zio.Warner
	zctx       *zed.Context

	// synchronization primitives
	results chan result
	readWg  sync.WaitGroup
	watchWg sync.WaitGroup
}

func New(zctx *zed.Context, dir string, opts anyio.ReaderOpts, warner zio.Warner, globs ...string) (*Tailer, error) {
	dir = filepath.Clean(dir)
	tailer, err := tail.TailDir(dir, globs...)
	if err != nil {
		return nil, err
	}
	if warner == nil {
		warner = nopWarner{}
	}
	r := &Tailer{
		opts:    opts,
		readers: make(map[string]*tail.File),
		results: make(chan result, 5),
		tailer:  tailer,
		warner:  warner,
		zctx:    zctx,
	}
	go r.start()
	return r, nil
}

type nopWarner struct{}

func (nopWarner) Warn(_ string) error { return nil }

type result struct {
	zv  *zed.Value
	err error
}

func (t *Tailer) start() {
	var err error
	t.watchWg.Add(1)
	for {
		ev, ok := <-t.tailer.Events
		// Watcher closed. Enstruct all go routines to stop tailing files so
		// they read remaining data then exit.
		if !ok {
			forceClose := atomic.LoadUint32(&t.forceClose) == 1
			t.stopReaders(forceClose)
			break
		}
		if ev.Err != nil {
			err = ev.Err
			t.stopReaders(true)
			break
		}
		if ev.Op.Exists() {
			if terr := t.tailFile(ev.Name); terr != nil {
				err = terr
				t.stopReaders(true)
				break
			}
		}
	}
	t.watchWg.Done()
	// Wait for all tail go routines to stop. We are about to close the results
	// channel and do not want a write to closed channel panic.
	t.readWg.Wait()
	// signfy EOS and close channel
	t.results <- result{err: err}
	close(t.results)
}

// stopReaders instructs all open TFile to stop tailing their respective files.
// If close is set to false, the readers will read through the remaining data
// in their files before emitting EOF. If close is set to true, the file
// descriptors will be closed and no further data will be read.
func (t *Tailer) stopReaders(close bool) {
	for _, r := range t.readers {
		if close {
			r.Close()
		} else {
			r.Stop()
		}
	}
}

func (t *Tailer) tailFile(file string) error {
	if _, ok := t.readers[file]; ok {
		return nil
	}
	f, err := tail.NewFile(file)
	if errors.Is(tail.ErrIsDir, err) {
		return nil
	}
	if err != nil {
		return err
	}

	t.readers[file] = f
	t.readWg.Add(1)
	go func() {
		defer t.readWg.Done()

		zf, err := anyio.NewFile(t.zctx, f, file, t.opts)
		if err != nil {
			f.Close()
			t.warner.Warn(fmt.Sprintf("%s: %v", filepath.Base(file), err))
			return
		}
		defer zf.Close()

		var zr zio.Reader = zf
		if t.warner != nil {
			zr = zio.NewWarningReader(zr, t.warner)
		}

		for {
			val, err := zr.Read()
			if err != nil {
				t.results <- result{err: err}
				return
			}
			if val == nil {
				return
			}
			// Copy because we may read the next value before
			// Tailer.Read's caller has finished with this one.
			t.results <- result{zv: val.Copy()}
		}
	}()
	return nil
}

func (t *Tailer) Read() (*zed.Value, error) {
	res, ok := <-t.results
	if !ok {
		// already closed return EOS
		return nil, nil
	}
	if res.err != nil {
		t.tailer.Stop() // exits loop
		// drain results
		for range t.results {
		}
	}
	return res.zv, res.err
}

// Stop instructs the directory watcher and indiviual file watchers to stop
// watching for changes. Read will emit EOS when the remaining unread data
// in files has been read.
func (t *Tailer) Stop() error {
	err := t.tailer.Stop()
	t.watchWg.Wait()
	return err
}

func (t *Tailer) Close() error {
	atomic.StoreUint32(&t.forceClose, 1)
	err := t.tailer.Stop()
	t.readWg.Wait()
	return err
}
