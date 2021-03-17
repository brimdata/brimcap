package analyzer

import (
	_ "embed"
	"errors"
	"flag"
	"io"

	"github.com/brimsec/zq/compiler"
	"github.com/brimsec/zq/zng/resolver"
)

//go:embed suricata.zed
var suricatashaper string

var ZeekExecScriptDisableSomeLogs = `
event zeek_init() {
	Log::disable_stream(PacketFilter::LOG);
	Log::disable_stream(LoadedScripts::LOG);
}`

var ZeekExecScriptLoadPackages = `
	@load packages
`

var (
	DefaultZeek = Config{
		Args: []string{"-C", "-r", "-", "--exec", ZeekExecScriptLoadPackages, "--exec", ZeekExecScriptDisableSomeLogs, "local"},
		Cmd:  "zeek",
	}
	DefaultSuricata = Config{
		Args:   []string{"-r", "/dev/stdin"},
		Cmd:    "suricata",
		Globs:  []string{"*.json"},
		Shaper: compiler.MustParseProc(suricatashaper),
	}
)

type Flags struct {
	configs  []Config
	suricata bool
	zeek     bool
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
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

func (f *Flags) Open(pcapfile io.Reader) (Analyzer, error) {
	return Multi(resolver.NewContext(), pcapfile, f.configs...)
}
