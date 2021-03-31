package root

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/zed/pkg/charm"
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
	cli cli.Flags
	// Child is set by the select Child command.
	Child ChildCmd
	JSON  bool
}

func init() {
	Brimcap.Add(charm.Help)
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{}
	c.cli.SetFlags(f)
	isterm := term.IsTerminal(int(os.Stdout.Fd()))
	f.BoolVar(&c.JSON, "json", !isterm, "encode stderr in json")
	return c, nil
}

type ChildCmd interface {
	Exec([]string) error
}

func (c *Command) Cleanup() {
	c.cli.Cleanup()
}

func (c *Command) Init(all ...cli.Initializer) error {
	return c.cli.Init(all...)
}

func (c *Command) Run(args []string) error {
	if c.Child != nil {
		return c.writeError(c.Child.Exec(args))
	}
	defer c.cli.Cleanup()
	if err := c.cli.Init(); err != nil {
		return c.writeError(err)
	}
	return Brimcap.Exec(c, []string{"help"})
}

type MsgError struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

func (c *Command) writeError(err error) error {
	if err == nil {
		return nil
	}
	if c.JSON {
		json.NewEncoder(os.Stderr).Encode(MsgError{Type: "error", Error: err.Error()})
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
	return err
}
