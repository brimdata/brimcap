package analyzer

import (
	"fmt"
	"os"

	"github.com/brimdata/zed/zio/anyio"
	"go.uber.org/multierr"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Args       []string         `yaml:"args"`
	Cmd        string           `yaml:"cmd"`
	Globs      []string         `yaml:"globs"`
	ReaderOpts anyio.ReaderOpts `yaml:"-"`
	Shaper     string           `yaml:"shaper"`
	StdoutPath string           `yaml:"stdout"`
	StderrPath string           `yaml:"stderr"`
	// WorkDir if set uses the provided directory as the working directory for
	// the launched analyzer process. Normally a temporary directory is created
	// then deleted when the process is complete. If WorkDir is set the working
	// directory will not be deleted.
	WorkDir string `yaml:"workdir"`
}

func (c Config) validate() error {
	if c.WorkDir == "" {
		return fmt.Errorf("%s: workdir not set", c.Cmd)
	}
	return nil
}

func LoadYAMLConfigFile(path string) ([]Config, error) {
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

type Configs []Config

func (cs Configs) Validate() (merr error) {
	for _, config := range cs {
		if err := config.validate(); err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	return merr
}
