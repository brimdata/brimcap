package analyze

import (
	"errors"
	"flag"
	"os"
	"time"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cli/analyzecli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/zed/cli/outputflags"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/pkg/storage"
	"golang.org/x/term"
)

var Analyze = &charm.Spec{
	Name:  "analyze",
	Usage: "analyze [options] pcap",
	Short: "analyze a pcap and emit a stream of ZNG records",
	Long: `
The analyze command runs a pcap file or stream through multiple analyzer 
processes (for now this is Zeek and Suricata) and emits the generated logs from
these processes. Brimcap is built on top of the Zed system
(https://github.com/brimdata/zed), so the logs can be written into a variety of
structured log formats.

For those familiar with zq (https://github.com/brimdata/zed/cmd/zq), logs can
written as ZNG or ZSON, then use zq to efficiently search through them.
Additionally logs can also be written as NDJSON and then operated on using jq
(https://stedolan.github.io/jq/).

To analyze a pcap file and write the data as ZSON records to stdout, simply run:

brimcap analyze -z sample.pcap
`,
	New: New,
}

func init() {
	root.Brimcap.Add(Analyze)
}

type Command struct {
	*root.Command
	analyzecli.Display
	analyzeflags analyzecli.Flags
	nostats      bool
	out          outputflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.analyzeflags.SetFlags(f)
	c.out.SetFlags(f)
	f.BoolVar(&c.nostats, "nostats", false, "do not display stats in stderr")
	return c, nil
}

// json:
// - always display stats in stderr (except if -nostats is enabled)
// status line display stats iff:
// - -o is a file: display stats
// - -o is stdout and stdout is NOT a terminal: display stats

func (c *Command) Run(args []string) (err error) {
	if len(args) != 1 {
		return errors.New("expected 1 pcapfile arg")
	}
	ctx, cleanup, err := c.InitWithContext(&c.out, &c.analyzeflags)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := c.AddRunnersToPath(); err != nil {
		return err
	}
	emitter, err := c.out.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	defer emitter.Close()
	pcapfile, err := cli.OpenFileArg(args[0])
	if err != nil {
		return err
	}
	defer pcapfile.Close()
	info, err := pcapfile.Stat()
	if err != nil {
		return err
	}
	if root.LogJSON {
		c.Display = analyzecli.JSONDisplay(!c.nostats, info.Size(), nano.Span{})
	} else {
		tofile := c.out.FileName() != ""
		stats := !c.nostats && (tofile || !term.IsTerminal(int(os.Stdout.Fd())))
		c.Display = analyzecli.StatusLineDisplay(stats, info.Size(), nano.Span{})
	}
	defer c.Display.End()
	configs := c.analyzeflags.Configs
	return analyzer.Run(ctx, pcapfile, emitter, c, time.Second, configs...)
}
