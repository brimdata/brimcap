package tail

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

var ErrIsDir = errors.New("path is a directory")

type File struct {
	ctx     context.Context
	f       *os.File
	watcher *fsnotify.Watcher
}

func TailFile(name string) (*File, error) {
	return TailFileWithContext(context.Background(), name)
}

func TailFileWithContext(ctx context.Context, name string) (*File, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, ErrIsDir
	}
	f, err := os.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		f.Close()
		return nil, err
	}
	if err := watcher.Add(name); err != nil {
		f.Close()
		watcher.Close()
		return nil, err
	}
	return &File{ctx: ctx, f: f, watcher: watcher}, nil
}

func (t *File) Read(b []byte) (int, error) {
read:
	n, err := t.f.Read(b)
	if err == io.EOF {
		if n > 0 {
			return n, nil
		}
		if err := t.waitWrite(); err != nil {
			return 0, err
		}
		goto read
	}
	if errors.Is(err, os.ErrClosed) {
		err = io.EOF
	}
	return n, err
}

func (t *File) waitWrite() error {
	for {
		select {
		case ev, ok := <-t.watcher.Events:
			if !ok {
				return io.EOF
			}
			if ev.Op == fsnotify.Write {
				return nil
			}
		case err := <-t.watcher.Errors:
			return err
		case <-t.ctx.Done():
			return t.ctx.Err()
		}
	}
}

func (t *File) Stop() error {
	return t.watcher.Close()
}

func (t *File) Close() error {
	return t.f.Close()
}

type FileOp int

const (
	FileOpCreated FileOp = iota
	FileOpExisting
	FileOpRemoved
)

func (o FileOp) Exists() bool {
	return o == FileOpCreated || o == FileOpExisting
}

type FileEvent struct {
	Name string
	Op   FileOp
	Err  error
}

// Dir observes a directory and will emit events when files are added
// or removed. When open for the first time this will emit an event for
// every existing file.
type Dir struct {
	Events chan FileEvent

	dir     string
	globs   []string
	watched map[string]struct{}
	watcher *fsnotify.Watcher
}

func TailDir(dir string, globs ...string) (*Dir, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("provided path must be a directory")
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Dir{
		Events:  make(chan FileEvent),
		dir:     dir,
		globs:   globs,
		watched: make(map[string]struct{}),
		watcher: watcher,
	}
	if err := w.watcher.Add(w.dir); err != nil {
		return nil, err
	}
	go func() {
		err := w.run()
		if errc := w.watcher.Close(); err == nil {
			err = errc
		}
		if err != nil {
			w.Events <- FileEvent{Err: err}
		}
		close(w.Events)
	}()
	return w, nil
}

func (w *Dir) run() error {
	if err := w.poll(); err != nil {
		return err
	}
	for ev := range w.watcher.Events {
		switch {
		case ev.Op&fsnotify.Create == fsnotify.Create:
			if err := w.addFile(ev.Name); err != nil {
				return err
			}
		case ev.Op&fsnotify.Rename == fsnotify.Rename, ev.Op&fsnotify.Remove == fsnotify.Remove:
			if err := w.removeFile(ev.Name); err != nil {
				return err
			}
		}
	}
	// watcher has been closed, poll once more to make sure we haven't missed
	// any files due to race.
	return w.poll()
}

func (w *Dir) addFile(name string) error {
	p, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	base := filepath.Base(name)
	for _, glob := range w.globs {
		if ok, err := filepath.Match(glob, base); !ok {
			return err
		}
	}
	if _, ok := w.watched[p]; !ok {
		w.watched[p] = struct{}{}
		w.Events <- FileEvent{Name: p, Op: FileOpCreated}
	}
	return nil
}

func (w *Dir) removeFile(name string) error {
	p, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	if _, ok := w.watched[p]; ok {
		delete(w.watched, p)
		w.Events <- FileEvent{Name: p, Op: FileOpRemoved}
	}
	return nil
}

func (w *Dir) poll() error {
	infos, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		if err := w.addFile(filepath.Join(w.dir, info.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (w *Dir) Stop() error {
	return w.watcher.Close()
}
