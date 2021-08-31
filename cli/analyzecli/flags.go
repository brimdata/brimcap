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

func (f *Flags) LoadConfigs() ([]analyzer.Config, error) {
	var err error
	var configs []analyzer.Config
	if f.configPath != "" {
		if configs, err = analyzer.LoadYAMLConfigFile(f.configPath); err != nil {
			return nil, err
		}
	} else {
		if f.zeek {
			configs = append(configs, DefaultZeek)
		}
		if f.suricata {
			configs = append(configs, DefaultSuricata)
		}
	}
	if len(configs) == 0 {
		return nil, errors.New("at least one analyzer (zeek or suricata) must be enabled")
	}
	return configs, nil
}

// EnsureWorkDirs creates temporary directories and sets them for a config if
// WorkDir is not set. A list of any created temporary directory paths is
// returned.
func EnsureWorkDirs(configs []analyzer.Config) ([]string, error) {
	var tmpdirs []string
	var err error
	for i := range configs {
		if configs[i].WorkDir == "" {
			configs[i].WorkDir, err = os.MkdirTemp("", "brimcap-")
			if err != nil {
				break
			}
			tmpdirs = append(tmpdirs, configs[i].WorkDir)
		}
	}
	return tmpdirs, err
}
