package analyzecli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/zed/pkg/display"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/pkg/units"
)

type Display struct {
	analyzer analyzer.Interface
	display  interface {
		Run()
		Close()
	}
	pcapsize      int64
	start         nano.Ts
	span          nano.Span
	json          bool
	warningsCount int32

	warningsMu sync.Mutex
	warnings   map[string]int
}

func NewDisplay(json bool) *Display {
	return &Display{
		json:     json,
		start:    nano.Now(),
		warnings: make(map[string]int),
	}
}

func (a *Display) Run(analyzer analyzer.Interface, pcapsize int64, span nano.Span) {
	analyzer.WarningHandler(a)
	a.analyzer = analyzer
	a.pcapsize = pcapsize
	a.span = span
	if a.json {
		interval := time.Second
		a.display = newJSONDisplayer(a, interval)
	} else {
		interval := time.Millisecond * 100
		a.display = display.New(a, interval)
	}
	go a.display.Run()
}

func (a *Display) Display(w io.Writer) bool {
	status := a.status()
	if a.json {
		json.NewEncoder(os.Stderr).Encode(status)
		return true
	}

	if percent, ok := status.Completion(); ok {
		fmt.Fprintf(w, "%5.1f%% %s/%s ", percent, units.Bytes(status.PcapReadSize), units.Bytes(status.PcapTotalSize))
	} else {
		fmt.Fprintf(w, "%s ", units.Bytes(status.PcapReadSize))
	}

	fmt.Fprintf(w, "records=%d ", status.RecordsWritten)

	if status.WarningsCount > 0 {
		fmt.Fprintf(w, "warnings=%d", status.WarningsCount)
	}
	io.WriteString(w, "\n")
	return true
}

func (a *Display) printWarnings() {
	count := atomic.LoadInt32(&a.warningsCount)
	if len(a.warnings) > 0 {
		fmt.Fprintf(os.Stderr, "%d warnings occurred while parsing log data:\n", count)
	}
	for msg, count := range a.warnings {
		fmt.Fprintf(os.Stderr, "    %s: x%d\n", msg, count)
	}
}

func (a *Display) Warn(msg string) error {
	if a.json {
		return json.NewEncoder(os.Stderr).Encode(MsgWarning{
			Type:    "warning",
			Warning: msg,
		})
	}
	a.warningsMu.Lock()
	a.warnings[msg]++
	a.warningsMu.Unlock()
	atomic.AddInt32(&a.warningsCount, 1)
	return nil
}

type MsgWarning struct {
	Type    string `json:"type"`
	Warning string `json:"warning"`
}

type MsgStatus struct {
	Type           string     `json:"type"`
	StartTime      nano.Ts    `json:"start_time"`
	UpdateTime     nano.Ts    `json:"update_time"`
	PcapTotalSize  int64      `json:"pcap_total_size"`
	PcapReadSize   int64      `json:"pcap_read_size"`
	RecordsWritten int64      `json:"records_written"`
	WarningsCount  int32      `json:"-"`
	Span           *nano.Span `json:"span,omitempty"`
}

func (m MsgStatus) Completion() (float64, bool) {
	if m.PcapTotalSize == 0 {
		return 0, false
	}
	return float64(m.PcapReadSize) / float64(m.PcapTotalSize) * 100, true
}

func (a *Display) status() MsgStatus {
	span := new(nano.Span)
	if a.span.Dur > 0 {
		span = &a.span
	}
	return MsgStatus{
		Type:           "status",
		StartTime:      a.start,
		UpdateTime:     nano.Now(),
		PcapTotalSize:  a.pcapsize,
		PcapReadSize:   a.analyzer.BytesRead(),
		RecordsWritten: a.analyzer.RecordsRead(),
		Span:           span,
		WarningsCount:  atomic.LoadInt32(&a.warningsCount),
	}
}

func (a *Display) Close() error {
	if a.display != nil {
		a.display.Close()
	}
	a.printWarnings()
	return nil
}
