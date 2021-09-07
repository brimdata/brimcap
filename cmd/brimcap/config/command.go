package config

import (
	"flag"
	"os"

	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/zed/pkg/charm"
	"gopkg.in/yaml.v3"
)

var Config = &charm.Spec{
	Name:  "config",
	Usage: "config [options]",
	Short: "config XXX",
	New:   New,
}

func init() {
	root.Brimcap.Add(Config)
}

type Command struct {
	*root.Command
	config cli.ConfigFlags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	err := c.config.SetFlags(f)
	return c, err
}

func (c *Command) Run(args []string) (err error) {
	if err := c.config.Validate(); err != nil {
		return err
	}
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(c.config.Config)
}
