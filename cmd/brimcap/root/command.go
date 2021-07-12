package root

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

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
is built on top of the flexible Zed system (https://github.com/brimdata/zed),
so the logs can be written into a variety of structured log formats.

For those familiar with zq (https://github.com/brimdata/zed/tree/main/cmd/zed#zq),
logs can written as ZNG or ZSON, then use zq to efficiently search through
them. Logs of these same formats could also be loaded into a Zed lake
(https://github.com/brimdata/zed/blob/main/docs/lake/design.md) using
zapi (https://github.com/brimdata/zed/tree/main/cmd/zed#zapi), which allows
for viewing the logs in the rich GUI of the Brim desktop app
(https://github.com/brimdata/brim).

Additionally logs can also be written as ndjson and then operated on using jq
(https://stedolan.github.io/jq/).

The brimcap index command can be used to index pcap files for
flow extraction via the brimcap search command.
`,
	New: New,
}

var LogJSON bool

type Command struct {
	charm.Command
	cli cli.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{}
	c.cli.SetFlags(f)
	isterm := term.IsTerminal(int(os.Stdout.Fd()))
	f.BoolVar(&LogJSON, "json", !isterm, "encode stderr in json")
	return c, nil
}

func (c *Command) Cleanup() {
	c.cli.Cleanup()
}

func (c *Command) InitWithContext(all ...cli.Initializer) (context.Context, func(), error) {
	if err := c.cli.Init(all...); err != nil {
		return nil, nil, err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	cleanup := func() {
		cancel()
		c.cli.Cleanup()
	}
	return ctx, cleanup, nil
}

func (c *Command) Init(all ...cli.Initializer) (func(), error) {
	if err := c.cli.Init(all...); err != nil {
		return nil, err
	}
	return c.cli.Cleanup, nil
}

func (c *Command) Run(args []string) error {
	defer c.cli.Cleanup()
	if err := c.cli.Init(); err != nil {
		return err
	}
	return charm.NeedHelp
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

func LogError(err error) error {
	if err == nil {
		return nil
	}
	if LogJSON {
		json.NewEncoder(os.Stderr).Encode(MsgError{Type: "error", Error: err.Error()})
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
	return err
}
