package analyzecli

import (
	_ "embed"
	"errors"
	"flag"
	"os"

	"github.com/brimdata/brimcap/analyzer"
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
		Shaper: suricatashaper,
	}
)

type Flags struct {
	Configs        []analyzer.Config
	configPath     string
	suricata       bool
	suricataStderr string
	suricataStdout string
	zeek           bool
	zeekStderr     string
	zeekStdout     string
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&f.configPath, "config", "", "path to YAML configuration file")
	fs.BoolVar(&f.suricata, "suricata", true, "run suricata pcap analyzer")
	fs.StringVar(&DefaultSuricata.StderrPath, "suricata.stderr", "", "write suricata process stderr to path")
	fs.StringVar(&DefaultSuricata.StdoutPath, "suricata.stdout", "", "write suricata process stderr to path")
	fs.BoolVar(&f.zeek, "zeek", true, "run zeek pcap analyzer")
	fs.StringVar(&DefaultZeek.StderrPath, "zeek.stderr", "", "write zeek process stderr to path")
	fs.StringVar(&DefaultZeek.StdoutPath, "zeek.stdout", "", "write zeek process stderr to path")
}

func (f *Flags) Init() (err error) {
	if f.configPath != "" {
		if f.Configs, err = analyzer.LoadYAMLConfigFile(f.configPath); err != nil {
			return err
		}
	} else {
		if f.zeek {
			f.Configs = append(f.Configs, DefaultZeek)
		}
		if f.suricata {
			f.Configs = append(f.Configs, DefaultSuricata)
		}
	}
	if len(f.Configs) == 0 {
		return errors.New("at least one analyzer (zeek or suricata) must be enabled")
	}
	for i := range f.Configs {
		if f.Configs[i].WorkDir == "" {
			f.Configs[i].WorkDir, err = os.MkdirTemp("", "brimcap-")
			if err != nil {
				return err
			}
		}
	}
	return nil
}
