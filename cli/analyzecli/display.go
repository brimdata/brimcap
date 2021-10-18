package analyzecli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

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

func StatusLineDisplay(stats bool, pcapsize int64, span nano.Span) Display {
	d := &statusLineDisplay{
		bypass:   os.Stderr,
		pcapsize: pcapsize,
		stats:    stats,
	}
	if stats {
		d.live = uilive.New()
		d.live.Out = os.Stderr
		d.live.Start()
		d.bypass = d.live.Bypass()
	}
	return d
}

type statusLineDisplay struct {
	bypass   io.Writer
	live     *uilive.Writer
	pcapsize int64
	stats    bool
}

func (d *statusLineDisplay) Warn(msg string) error {
	fmt.Fprintln(d.bypass, msg)
	return nil
}

func (d *statusLineDisplay) Stats(stats analyzer.Stats) error {
	if !d.stats {
		return nil
	}
	if d.pcapsize > 0 {
		percent := (float64(stats.BytesRead) / float64(d.pcapsize)) * 100
		fmt.Fprintf(d.live, "%5.1f%% %s/%s ", percent, units.Bytes(stats.BytesRead).Abbrev(), units.Bytes(d.pcapsize).Abbrev())
	} else {
		fmt.Fprintf(d.live, "%s ", units.Bytes(stats.BytesRead).Abbrev())
	}
	fmt.Fprintf(d.live, "values=%d ", stats.ValuesWritten)
	io.WriteString(d.live, "\n")
	return d.live.Flush()
}

func (d *statusLineDisplay) End() {
	if d.live != nil {
		d.live.Stop()
	}
}

func JSONDisplay(stats bool, pcapsize int64, span nano.Span) Display {
	var s *nano.Span
	if span.Dur > 0 {
		s = &span
	}
	return &jsonDisplay{
		encoder:  json.NewEncoder(os.Stderr),
		pcapsize: pcapsize,
		span:     s,
		stats:    stats,
	}
}

type jsonDisplay struct {
	encoder  *json.Encoder
	pcapsize int64
	span     *nano.Span
	stats    bool
}

func (j *jsonDisplay) Warn(msg string) error {
	return j.encoder.Encode(MsgWarning{
		Type:    "warning",
		Warning: msg,
	})
}

func (j *jsonDisplay) Stats(stats analyzer.Stats) error {
	if !j.stats {
		return nil
	}
	return j.encoder.Encode(MsgStatus{
		Type:          "status",
		Ts:            nano.Now(),
		PcapReadSize:  stats.BytesRead,
		PcapTotalSize: j.pcapsize,
		Span:          j.span,
		ValuesWritten: stats.ValuesWritten,
	})
}

func (*jsonDisplay) End() {}

type MsgWarning struct {
	Type    string `json:"type"`
	Warning string `json:"warning"`
}

type MsgStatus struct {
	Type          string     `json:"type"`
	Ts            nano.Ts    `json:"ts"`
	PcapReadSize  int64      `json:"pcap_read_size"`
	PcapTotalSize int64      `json:"pcap_total_size"`
	Span          *nano.Span `json:"span,omitempty"`
	ValuesWritten int64      `json:"values_written"`
}
