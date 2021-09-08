package brimcap

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/brimdata/brimcap/analyzer"
	"gopkg.in/yaml.v3"
)

//go:embed suricata.zed
var suricatashaper string

var DefaultConfig = Config{
	Analyzers: []analyzer.Config{
		{
			Name:   "suricata",
			Cmd:    "suricatarunner",
			Globs:  []string{"*.json"},
			Shaper: suricatashaper,
		},
		{
			Name: "zeek",
			Cmd:  "zeekrunner",
		},
	},
}

type Config struct {
	RootPath  string            `yaml:"root,omitempty"`
	Analyzers []analyzer.Config `yaml:"analyzers,omitempty"`
}

func LoadConfigYAML(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("error loading config file: %w", err)
	}
	var c Config
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		err = fmt.Errorf("error loading config file: %w", err)
	}
	return c, err
}

func (c Config) Root() Root { return Root(c.RootPath) }

func (c Config) Validate() error {
	return analyzer.Configs(c.Analyzers).Validate()
}
