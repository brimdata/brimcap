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
	"github.com/brimdata/zed/cmd/zapi/format"
	"github.com/brimdata/zed/pkg/display"
	"github.com/brimdata/zed/pkg/nano"
)

type Display struct {
	analyzer analyzer.Interface
	display  interface {
		Run()
		Close()
	}
	pcapsize      int64
	start         nano.Ts
	json          bool
	warnings      map[string]int
	warningsCount int32
	warningsMu    sync.Mutex
}

func NewDisplay(json bool, pcapsize int64) *Display {
	return &Display{
		json:     json,
		pcapsize: pcapsize,
		start:    nano.Now(),
	}
}

func (a *Display) Run(analyzer analyzer.Interface) {
	analyzer.WarningHandler(a)
	a.analyzer = analyzer
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
		fmt.Fprintf(w, "%5.1f%% %s/%s ", percent, format.Bytes(status.PcapReadSize), format.Bytes(status.PcapSize))
	} else {
		fmt.Fprintf(w, "%s ", format.Bytes(status.PcapReadSize))
	}

	fmt.Fprintf(w, "records=%d ", status.RecordsWritten)

	if status.WarningsCount > 0 {
		fmt.Fprintf(w, "warnings=%d", status.WarningsCount)
	}
	io.WriteString(w, "\n")
	return true
}

func (a *Display) jsonDisplay(w io.Writer) bool {
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
	if a.warnings == nil {
		a.warnings = make(map[string]int)
	}
	a.warnings[msg]++
	atomic.AddInt32(&a.warningsCount, 1)
	a.warningsMu.Unlock()
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
	PcapSize       int64      `json:"pcap_total_size" unit:"bytes"`
	PcapReadSize   int64      `json:"pcap_read_size" unit:"bytes"`
	RecordsWritten int64      `json:"records_written"`
	WarningsCount  int32      `json:"-"`
	Span           *nano.Span `json:"span,omitempty"`
}

func (m MsgStatus) Completion() (float64, bool) {
	if m.PcapSize == 0 {
		return 0, false
	}
	return float64(m.PcapReadSize) / float64(m.PcapSize) * 100, true
}

func (a *Display) status() MsgStatus {
	return MsgStatus{
		Type:           "status",
		StartTime:      a.start,
		UpdateTime:     nano.Now(),
		PcapSize:       a.pcapsize,
		PcapReadSize:   a.analyzer.BytesRead(),
		RecordsWritten: a.analyzer.RecordsRead(),
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
