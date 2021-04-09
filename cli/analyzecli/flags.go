package analyzecli

import (
	_ "embed"
	"errors"
	"flag"

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
	configPath     string
	Configs        []analyzer.Config
	suricata       bool
	suricataStderr string
	suricataStdout string
	zeek           bool
	zeekStderr     string
	zeekStdout     string
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&f.configPath, "config", "", "path to configuration yaml file")
	fs.BoolVar(&f.suricata, "suricata", true, "run suricata pcap analyzer")
	fs.StringVar(&DefaultSuricata.StderrPath, "suricata.stderr", "", "write suricata process stderr to path")
	fs.StringVar(&DefaultSuricata.StdoutPath, "suricata.stdout", "", "write suricata process stderr to path")
	fs.BoolVar(&f.zeek, "zeek", true, "run zeek pcap analyzer")
	fs.StringVar(&DefaultZeek.StderrPath, "zeek.stderr", "", "write zeek process stderr to path")
	fs.StringVar(&DefaultZeek.StdoutPath, "zeek.stdout", "", "write zeek process stderr to path")
}

func (f *Flags) Init() (err error) {
	if f.configPath != "" {
		f.Configs, err = analyzer.LoadYamlConfigFile(f.configPath)
		return err
	}
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
