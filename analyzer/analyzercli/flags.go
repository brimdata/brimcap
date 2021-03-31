package analyzercli

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zng/resolver"
	"golang.org/x/term"
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

type Flags struct {
	configs  []analyzer.Config
	json     bool
	suricata bool
	zeek     bool
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	isterm := term.IsTerminal(int(os.Stdout.Fd()))
	fs.BoolVar(&f.json, "json", !isterm, "write json progress updates to stderr")
	fs.BoolVar(&f.suricata, "suricata", true, "enables/disables suricata pcap analyzer")
	fs.BoolVar(&f.zeek, "zeek", true, "enables/disables zeek pcap analyzer")
}

func (f *Flags) Init() error {
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

func (f *Flags) Open(ctx context.Context, args []string) (*CLI, error) {
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

	cli := &CLI{
		Interface:    analyzer.CombinerWithContext(ctx, resolver.NewContext(), pcapfile, f.configs...),
		jsonprogress: f.json,
		pcapfile:     pcapfile,
		pcapsize:     pcapsize,
		start:        nano.Now(),
		warnings:     make(map[string]int64),
	}
	cli.Interface.WarningHandler(cli)
	return cli, nil
}
