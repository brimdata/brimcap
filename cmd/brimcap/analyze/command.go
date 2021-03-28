package analyze

import (
	"errors"
	"flag"
	"os"

	"github.com/brimsec/brimcap/cmd/brimcap/root"
	"github.com/brimsec/zq/cli/outputflags"
	"github.com/brimsec/zq/emitter"
	"github.com/brimsec/zq/pkg/signalctx"
	"github.com/brimsec/zq/zbuf"
	"github.com/mccanne/charm"
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
	emitter      emitter.Emitter
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

	c.emitter, err = c.out.Open(ctx)
	if err != nil {
		return err
	}
	defer c.emitter.Close()

	c.analyzer, err = c.analyzeflags.Open(ctx, args)
	if err != nil {
		return err
	}

	// If not emitting to stdio write stats to stderr.
	if !c.emitter.IsStdio() {
		c.analyzer.RunDisplay()
	}

	err = zbuf.CopyWithContext(ctx, c.emitter, c.analyzer)
	if aerr := c.analyzer.Close(); err == nil {
		err = aerr
	}
	return err
}
