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
	"github.com/brimdata/zed/zng/resolver"
)

var Analyze = &charm.Spec{
	Name:  "analyze",
	Usage: "analyze [options] [ pcapfile ]",
	Short: "",
	Long:  ``,
	New:   New,
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

	pcapfile, pcapsize, err := cli.OpenFileArg(args[0])
	if err != nil {
		return err
	}
	defer pcapfile.Close()

	display := analyzecli.NewDisplay(c.JSON)
	zctx := resolver.NewContext()
	analyzer := analyzer.CombinerWithContext(ctx, zctx, pcapfile, c.analyzeflags.Configs...)

	// If not emitting to stdio write stats to stderr.
	if c.out.FileName() != "" {
		display := analyzecli.NewDisplay(c.JSON)
		display.Run(analyzer, pcapsize, nano.Span{})
		defer display.Close()
	}

	err = zbuf.CopyWithContext(ctx, emitter, analyzer)
	if aerr := analyzer.Close(); err == nil {
		err = aerr
	}
	display.Close()
	return err
}
