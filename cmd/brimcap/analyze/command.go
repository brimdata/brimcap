package analyze

import (
	"errors"
	"flag"
	"os"

	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/zed/cli/outputflags"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/signalctx"
	"github.com/brimdata/zed/zbuf"
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
	analyzeflags root.AnalyzerFlags
	analyzer     *root.AnalyzerCLI
	emitter      zbuf.WriteCloser
	out          outputflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.analyzeflags.SetFlags(f)
	c.out.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) (err error) {
	if len(args) != 1 {
		return errors.New("expected 1 pcapfile arg")
	}

	if err := c.Init(&c.out, &c.analyzeflags); err != nil {
		return err
	}
	defer c.Cleanup()

	ctx, cancel := signalctx.New(os.Interrupt)
	defer cancel()

	emitter, err := c.out.Open(ctx)
	if err != nil {
		return err
	}
	defer emitter.Close()

	c.analyzer, err = c.analyzeflags.Open(ctx, args)
	if err != nil {
		return err
	}

	// If not emitting to stdio write stats to stderr.
	if c.out.FileName() != "" {
		c.analyzer.RunDisplay()
	}

	err = zbuf.CopyWithContext(ctx, emitter, c.analyzer)
	if aerr := c.analyzer.Close(); err == nil {
		err = aerr
	}
	return err
}
