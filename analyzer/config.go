package analyzer

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/brimdata/zed/zio/anyio"
	"go.uber.org/multierr"
)

type Config struct {
	Args []string `yaml:"args,omitempty"`
	// Cmd is the command to run for this analyzer (required).
	Cmd      string   `yaml:"cmd"`
	Disabled bool     `yaml:"disabled,omitempty"`
	Globs    []string `yaml:"globs,omitempty"`
	// Name is a unique selector for this analyzer (required).
	Name       string           `yaml:"name"`
	ReaderOpts anyio.ReaderOpts `yaml:"-"`
	Shaper     string           `yaml:"shaper,omitempty"`
	StdoutPath string           `yaml:"stdout,omitempty"`
	StderrPath string           `yaml:"stderr,omitempty"`
	// WorkDir if set uses the provided directory as the working directory for
	// the launched analyzer process. Normally a temporary directory is created
	// then deleted when the process is complete. If WorkDir is set the working
	// directory will not be deleted.
	WorkDir string `yaml:"workdir,omitempty"`
}

func (c *Config) SetFlags(fs *flag.FlagSet) {
	pre := fmt.Sprintf("analyzers.%s.", c.Name)
	fs.StringVar(&c.Cmd, pre+"cmd", c.Cmd, "command to run")
	fs.BoolVar(&c.Disabled, pre+"disabled", c.Disabled, "disable analyzer")
	fs.StringVar(&c.StdoutPath, pre+"stdout", c.StdoutPath, "write stdout to path")
	fs.StringVar(&c.StderrPath, pre+"stderr", c.StderrPath, "write stderr to path")
	fs.StringVar(&c.WorkDir, pre+"workdir", c.WorkDir, "working directory")
}

func (c Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%s: name value must be set", c.getName())
	}
	if c.Cmd == "" {
		return fmt.Errorf("%s: cmd value must be set", c.getName())
	}
	return nil
}

// Name returns the Name field if set, otherwise it returns the name of the cmd.
func (c Config) getName() string {
	if c.Name != "" {
		return c.Name
	}
	return filepath.Base(c.Cmd)
}

type Configs []Config

func (cs Configs) Validate() (merr error) {
	names := make(map[string]struct{})
	for _, config := range cs {
		if err := config.Validate(); err != nil {
			merr = multierr.Append(merr, err)
		} else {
			if _, ok := names[config.Name]; ok {
				merr = multierr.Append(merr, fmt.Errorf("%s: name field must be unique", config.getName()))
			}
			names[config.Name] = struct{}{}
		}
	}
	return merr
}

func (cs Configs) removeDisabled() Configs {
	var confs Configs
	for _, config := range cs {
		if !config.Disabled {
			confs = append(confs, config)
		}
	}
	return confs
}

func (cs Configs) ensureWorkDirs() (func(), error) {
	var dir string
	for i := range cs {
		if cs[i].WorkDir == "" {
			if dir == "" {
				var err error
				dir, err = os.MkdirTemp("", "brimcap-")
				if err != nil {
					return nil, err
				}
			}
			cs[i].WorkDir = filepath.Join(dir, strconv.Itoa(i))
			if err := os.Mkdir(cs[i].WorkDir, 0700); err != nil {
				os.RemoveAll(dir)
				return nil, err
			}
		}
	}
	return func() {
		if dir != "" {
			os.RemoveAll(dir)
		}
	}, nil
}
