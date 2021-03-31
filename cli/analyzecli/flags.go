package analyzecli

import (
	_ "embed"
	"errors"
	"flag"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/zed/compiler"
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
	Configs  []analyzer.Config
	suricata bool
	zeek     bool
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&f.suricata, "suricata", true, "enables/disables suricata pcap analyzer")
	fs.BoolVar(&f.zeek, "zeek", true, "enables/disables zeek pcap analyzer")
}

func (f *Flags) Init() error {
	if f.zeek {
		f.Configs = append(f.Configs, DefaultZeek)
	}
	if f.suricata {
		f.Configs = append(f.Configs, DefaultSuricata)
	}
	if len(f.Configs) == 0 {
		return errors.New("at least one analyzer (zeek or suricata) must be enabled")
	}
	return nil
}
