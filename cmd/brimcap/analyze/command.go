package analyze

import (
	"errors"
	"flag"
	"os"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cli/analyzecli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/zed/cli/outputflags"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/pkg/signalctx"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zson"
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
	analyzeflags analyzecli.Flags
	emitter      zbuf.WriteCloser
	out          outputflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.Command.Child = c
	c.analyzeflags.SetFlags(f)
	c.out.SetFlags(f)
	return c, nil
}

func (c *Command) Exec(args []string) (err error) {
	if len(args) != 1 {
		return errors.New("expected 1 pcapfile arg")
	}

	if err := c.Init(&c.out, &c.analyzeflags); err != nil {
		return err
	}
	defer c.Cleanup()

	if err := c.AddRunnersToPath(); err != nil {
		return err
	}

	ctx, cancel := signalctx.New(os.Interrupt)
	defer cancel()

	emitter, err := c.out.Open(ctx)
	if err != nil {
		return err
	}
	defer emitter.Close()

	pcapfile, err := cli.OpenFileArg(args[0])
	if err != nil {
		return err
	}
	defer pcapfile.Close()

	display := analyzecli.NewDisplay(c.JSON)
	zctx := zson.NewContext()
	analyzer := analyzer.CombinerWithContext(ctx, zctx, pcapfile, c.analyzeflags.Configs...)

	// If not emitting to stdio write stats to stderr.
	if c.out.FileName() != "" {
		stat, err := pcapfile.Stat()
		if err != nil {
			return err
		}
		display := analyzecli.NewDisplay(c.JSON)
		display.Run(analyzer, stat.Size(), nano.Span{})
		defer display.Close()
	}

	err = zbuf.CopyWithContext(ctx, emitter, analyzer)
	if aerr := analyzer.Close(); err == nil {
		err = aerr
	}
	display.Close()
	return err
}
