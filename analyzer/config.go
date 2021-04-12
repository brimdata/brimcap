package analyzer

import (
	"os"

	"github.com/brimdata/zed/zio"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Args       []string       `yaml:"args"`
	Cmd        string         `yaml:"cmd"`
	Globs      []string       `yaml:"globs"`
	Launcher   Launcher       `yaml:"-"`
	ReaderOpts zio.ReaderOpts `yaml:"-"`
	Shaper     string         `yaml:"shaper"`
	StdoutPath string         `yaml:"stdout"`
	StderrPath string         `yaml:"stderr"`
}

func LoadYamlConfigFile(path string) ([]Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file := struct {
		Analyzers []Config `yaml:"analyzers"`
	}{}
	if err := yaml.Unmarshal(b, &file); err != nil {
		return nil, err
	}
	return file.Analyzers, nil
}
