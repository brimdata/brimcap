package analyzercli

import (
	"os"
	"time"

	"github.com/brimdata/zed/pkg/display"
)

type jsonDisplay struct {
	done      chan chan struct{}
	displayer display.Displayer
	dur       time.Duration
}

func newJSONDisplayer(displayer display.Displayer, dur time.Duration) *jsonDisplay {
	return &jsonDisplay{
		done:      make(chan chan struct{}),
		displayer: displayer,
		dur:       dur,
	}
}

func (d *jsonDisplay) Run() {
	ticker := time.NewTicker(d.dur)
	for {
		select {
		case <-ticker.C:
			d.displayer.Display(os.Stderr)
		case done := <-d.done:
			d.displayer.Display(os.Stderr)
			ticker.Stop()
			close(done)
			return
		}
	}
}

func (d *jsonDisplay) Close() {
	closed := make(chan struct{})
	d.done <- closed
	<-closed
}
