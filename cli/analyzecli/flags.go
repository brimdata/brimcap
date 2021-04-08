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

var (
	DefaultZeek = analyzer.Config{
		Cmd: "zeekrunner",
	}
	DefaultSuricata = analyzer.Config{
		Cmd:    "suricatarunner",
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
	fs.BoolVar(&f.suricata, "suricata", true, "run suricata pcap analyzer")
	fs.BoolVar(&f.zeek, "zeek", true, "run zeek pcap analyzer")
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
