package root

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/zed/cmd/zapi/format"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/pkg/display"
	"github.com/brimdata/zed/zng/resolver"
)

//go:embed suricata.zed
var suricatashaper string

var zeekscript = `
event zeek_init() {
    Log::disable_stream(PacketFilter::LOG);
    Log::disable_stream(LoadedScripts::LOG);
}`

var (
	DefaultZeek = analyzer.Config{
		Args: []string{"-C", "-r", "-", "--exec", "@load packages", "--exec", zeekscript, "local"},
		Cmd:  "zeek",
	}
	DefaultSuricata = analyzer.Config{
		Args:   []string{"-r", "/dev/stdin"},
		Cmd:    "suricata",
		Globs:  []string{"*.json"},
		Shaper: compiler.MustParseProc(suricatashaper),
	}
)

type AnalyzerFlags struct {
	configs  []analyzer.Config
	suricata bool
	zeek     bool
}

func (f *AnalyzerFlags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&f.suricata, "suricata", true, "enables/disables suricata pcap analyzer")
	fs.BoolVar(&f.zeek, "zeek", true, "enables/disables zeek pcap analyzer")
}

func (f *AnalyzerFlags) Init() error {
	if f.zeek {
		f.configs = append(f.configs, DefaultZeek)
	}
	if f.suricata {
		f.configs = append(f.configs, DefaultSuricata)
	}
	if len(f.configs) == 0 {
		return errors.New("at least one analyzer (zeek or suricata) must be enabled")
	}
	return nil
}

type AnalyzerCLI struct {
	analyzer.Interface
	display       *display.Display
	pcapfile      *os.File
	pcapsize      int64
	warningsMu    sync.Mutex
	warnings      map[string]int64
	warningsCount int32
}

func (a *AnalyzerCLI) Close() error {
	err := a.Interface.Close()
	if perr := a.pcapfile.Close(); err == nil {
		err = perr
	}
	if a.display != nil {
		a.display.Close()
	}
	a.printWarnings()
	return err
}

func (a *AnalyzerCLI) RunDisplay() {
	a.display = display.New(a, time.Millisecond*100)
	go a.display.Run()
}

func (a *AnalyzerCLI) Display(w io.Writer) bool {
	read := a.Interface.BytesRead()
	fmt.Fprintf(w, "records_read=%d ", a.Interface.RecordsRead())

	str := "pcap_bytes_read="
	if total := a.Pcapsize(); total != 0 {
		percent := float64(read) / float64(total) * 100
		str += fmt.Sprintf("%s/%s %5.1f%% ", format.Bytes(read), format.Bytes(total), percent)
	} else {
		str += format.Bytes(read)
	}
	io.WriteString(w, str)

	if warnings := a.WarningsCount(); warnings > 0 {
		fmt.Fprintf(w, "warnings=%d", warnings)
	}
	io.WriteString(w, "\n")
	return true
}

func (a *AnalyzerCLI) printWarnings() {
	count := atomic.LoadInt32(&a.warningsCount)
	if len(a.warnings) > 0 {
		fmt.Fprintf(os.Stderr, "%d warnings occurred while parsing log data:\n", count)
	}
	for msg, count := range a.warnings {
		fmt.Fprintf(os.Stderr, "    %s: x%d\n", msg, count)
	}
}

func (a *AnalyzerCLI) Warn(msg string) error {
	a.warningsMu.Lock()
	a.warnings[msg]++
	atomic.AddInt32(&a.warningsCount, 1)
	a.warningsMu.Unlock()
	return nil
}

func (a *AnalyzerCLI) Pcapsize() int64 {
	return a.pcapsize
}

func (a *AnalyzerCLI) WarningsCount() int32 {
	return atomic.LoadInt32(&a.warningsCount)
}

func (f *AnalyzerFlags) Open(ctx context.Context, args []string) (*AnalyzerCLI, error) {
	var pcapsize int64
	pcapfile := os.Stdin
	if path := args[0]; path != "-" {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error loading pcap file: %w", err)
		}
		pcapsize = info.Size()
		if pcapfile, err = os.Open(path); err != nil {
			return nil, fmt.Errorf("error loading pcap file: %w", err)
		}
	}

	cli := &AnalyzerCLI{
		Interface: analyzer.CombinerWithContext(ctx, resolver.NewContext(), pcapfile, f.configs...),
		pcapfile:  pcapfile,
		pcapsize:  pcapsize,
		warnings:  make(map[string]int64),
	}
	cli.Interface.WarningHandler(cli)
	return cli, nil
}
