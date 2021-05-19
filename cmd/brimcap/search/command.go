package search

import (
	"flag"
	"os"

	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/zed/pkg/charm"
)

var Search = &charm.Spec{
	Name:  "search",
	Usage: "search [options]",
	Short: "search for a connection in a list of pcaps",
	Long: `
The search command searches in parallel for a specific connection in a list of
indexed pcap files (generated using brimcap index -root) and writes the results
to a new pcap file or to standard output.
`,
	New: New,
}

func init() {
	root.Brimcap.Add(Search)
}

type Command struct {
	*root.Command
	outfile     string
	rootflags   cli.RootFlags
	searchflags cli.PcapSearchFlags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	f.StringVar(&c.outfile, "w", "-", "file to write to or stdout if -")
	c.rootflags.SetFlags(f)
	c.searchflags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) (err error) {
	ctx, cleanup, err := c.Command.InitWithContext(&c.rootflags, &c.searchflags)
	if err != nil {
		return err
	}
	defer cleanup()
	out := os.Stdout
	if c.outfile != "-" {
		out, err = os.Create(c.outfile)
		if err != nil {
			return err
		}
	}
	err = c.rootflags.Root.Search(ctx, c.searchflags.Search, out)
	if c.outfile != "-" {
		out.Close()
		if err != nil {
			os.Remove(c.outfile)
		}
	}
	return err
}
