package analyzecli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/pkg/units"
	"github.com/gosuri/uilive"
)

type Display interface {
	Warn(string) error
	Stats(analyzer.Stats) error
	End()
}

func NewDisplay(jsonOut bool, pcapsize int64, span nano.Span) Display {
	if jsonOut {
		var s *nano.Span
		if span.Dur > 0 {
			s = &span
		}
		return &jsonDisplay{
			encoder:  json.NewEncoder(os.Stderr),
			pcapsize: pcapsize,
			span:     s,
		}
	}
	return &statusLineDisplay{
		live:     uilive.New(),
		pcapsize: pcapsize,
		warnings: make(map[string]int),
	}
}

type statusLineDisplay struct {
	live          *uilive.Writer
	pcapsize      int64
	warnings      map[string]int
	warningsCount int64
	warningsMu    sync.Mutex
}

func (d *statusLineDisplay) Warn(msg string) error {
	d.warningsMu.Lock()
	d.warnings[msg]++
	d.warningsMu.Unlock()
	atomic.AddInt64(&d.warningsCount, 1)
	return nil
}

func (d *statusLineDisplay) Stats(stats analyzer.Stats) error {
	if d.pcapsize > 0 {
		percent := float64(stats.BytesRead) / float64(d.pcapsize)
		fmt.Fprintf(d.live, "%5.1f%% %s/%s ", percent, units.Bytes(stats.BytesRead), units.Bytes(d.pcapsize))
	} else {
		fmt.Fprintf(d.live, "%s ", units.Bytes(stats.BytesRead))
	}
	fmt.Fprintf(d.live, "records=%d ", stats.RecordsWritten)
	if warnings := atomic.LoadInt64(&d.warningsCount); warnings > 0 {
		fmt.Fprintf(d.live, "warnings=%d", warnings)
	}
	io.WriteString(d.live, "\n")
	return d.live.Flush()
}

func (d *statusLineDisplay) End() {
	d.live.Stop()
	warnings := atomic.LoadInt64(&d.warningsCount)
	if warnings == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%d warnings occurred while parsing log data:\n", warnings)
	d.warningsMu.Lock()
	for msg, count := range d.warnings {
		fmt.Fprintf(os.Stderr, "    %s: x%d\n", msg, count)
	}
	d.warningsMu.Unlock()
}

type jsonDisplay struct {
	encoder  *json.Encoder
	pcapsize int64
	span     *nano.Span
}

func (j *jsonDisplay) Warn(msg string) error {
	return j.encoder.Encode(MsgWarning{
		Type:    "warning",
		Warning: msg,
	})
}

func (j *jsonDisplay) Stats(stats analyzer.Stats) error {
	return j.encoder.Encode(MsgStatus{
		Type:           "status",
		Ts:             nano.Now(),
		PcapReadSize:   stats.BytesRead,
		PcapTotalSize:  j.pcapsize,
		RecordsWritten: stats.RecordsWritten,
		Span:           j.span,
	})
}

func (*jsonDisplay) End() {}

type MsgWarning struct {
	Type    string `json:"type"`
	Warning string `json:"warning"`
}

type MsgStatus struct {
	Type           string     `json:"type"`
	Ts             nano.Ts    `json:"ts"`
	PcapReadSize   int64      `json:"pcap_read_size"`
	PcapTotalSize  int64      `json:"pcap_total_size"`
	RecordsWritten int64      `json:"records_written"`
	Span           *nano.Span `json:"span,omitempty"`
}
