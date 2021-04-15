package root

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/zed/pkg/charm"
	"golang.org/x/term"
)

var Brimcap = &charm.Spec{
	Name:  "brimcap",
	Usage: "brimcap [global options] command [options] [arguments...]",
	Short: "search, analyze and inspect pcap files",
	Long: `
The Brimcap command provides utilties for searching, analyzing, and inspecting
pcap files. Most users will be interested in the brimcap analyze command, which
will read a pcap stream or file into multiple pcap analyzer processes (defaults
to Zeek and Suricata) and emits the generated logs from these processes. Brimcap
is built on top of the flexible zed library (https://github.com/brimdata/zed),
so the logs can be written into a variety of structured log formats.

For those familiar with zq (https://github.com/brimdata/zed/cmd/zq), logs can
written as zng or zson, then use zq to performantly search through them.
Additionally logs can also be written as ndjson and then operated on using jq
(https://stedolan.github.io/jq/).

The brimcap load command can be used to write logs into the Brim desktop app 
(https://github.com/brimdata/brim) for viewing logs in a rich ui environment.

The brimcap index can be used to index pcap files then efficiently searched
through using brimcap search.
`,
	New: New,
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

func (c *Command) AddRunnersToPath() error {
	execpath, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(execpath)
	pathEnv := os.Getenv("PATH")
	zeekPath := filepath.Join(dir, "zeek")
	suricataPath := filepath.Join(dir, "suricata")
	pathEnv = strings.Join([]string{pathEnv, zeekPath, suricataPath}, string(os.PathListSeparator))
	return os.Setenv("PATH", pathEnv)
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
