package root

import (
	"flag"
	"log"
	"os"

	"github.com/brimsec/zq/cli"
	"github.com/mccanne/charm"
	"golang.org/x/term"
)

var Brimcap = &charm.Spec{
	Name:  "brimcap",
	Usage: "brimcap [global options] command [options] [arguments...]",
	Short: "brimcap XXX",
	Long:  ``,
	New:   New,
}

type Command struct {
	charm.Command
	cli     cli.Flags
	NoFancy bool
}

func init() {
	Brimcap.Add(charm.Help)
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{
		NoFancy: !term.IsTerminal(int(os.Stdout.Fd())),
	}
	f.BoolVar(&c.NoFancy, "nofancy", c.NoFancy, "disable fancy CLI output (true if stdout is not a tty)")
	log.SetPrefix("brimcap") // XXX switch to zap
	c.cli.SetFlags(f)
	return c, nil
}

func (c *Command) Cleanup() {
	c.cli.Cleanup()
}

func (c *Command) Init(all ...cli.Initializer) error {
	return c.cli.Init(all...)
}

func (c *Command) Run(args []string) error {
	defer c.cli.Cleanup()
	if err := c.cli.Init(); err != nil {
		return err
	}
	return Brimcap.Exec(c, []string{"help"})
}
