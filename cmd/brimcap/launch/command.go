package launch

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"os/signal"

	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/zed/pkg/charm"
)

var Launch = &charm.Spec{
	Name:  "launch",
	Usage: "launch [options]",
	Short: "search for a connection and load results in wireshark",
	Long: `
The search command searches in parallel for a specific connection in a list of
indexed pcap files (generated using brimcap index -root) and loads any results
into wireshark. In order for brimcap launch to work wireshark
(https://wireshark.org/#download) must be installed.
`,
	New: New,
}

func init() {
	root.Brimcap.Add(Launch)
}

type Command struct {
	*root.Command
	rootflags   cli.RootFlags
	searchflags cli.PcapSearchFlags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.Command.Child = c
	c.rootflags.SetFlags(f)
	c.searchflags.SetFlags(f)
	return c, nil
}

func (c *Command) Init() error {
	_, err := exec.LookPath("wireshark")
	return err
}

func (c *Command) Exec(args []string) (err error) {
	if err := c.Command.Init(&c.rootflags, &c.searchflags, c); err != nil {
		return err
	}
	defer c.Cleanup()

	f, err := os.CreateTemp("", "brimcap-launch-")
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	err = c.rootflags.Root.Search(ctx, c.searchflags.Search, f)
	f.Close()
	if err != nil {
		return err
	}

	return exec.Command("wireshark", "-r", f.Name()).Start()
}
