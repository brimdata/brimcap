package analyze

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/brimsec/brimcap/analyzer"
	"github.com/brimsec/brimcap/cmd/brimcap/root"
	"github.com/brimsec/zq/cli/outputflags"
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
	analyze analyzer.Flags
	out     outputflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.analyze.SetFlags(f)
	c.out.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) (err error) {
	if len(args) != 1 {
		return errors.New("expected 1 pcapfile arg")
	}

	if err := c.Init(&c.out, &c.analyze); err != nil {
		return err
	}
	defer c.Cleanup()

	zw, err := c.out.Open(context.Background())
	if err != nil {
		return err
	}
	defer zw.Close()

	pcapfile := os.Stdin
	if args[0] != "-" {
		if pcapfile, err = os.Open(args[0]); err != nil {
			return fmt.Errorf("error loading pcap file: %w", err)
		}
	}
	defer pcapfile.Close()

	reader, err := c.analyze.Open(pcapfile)
	if err != nil {
		return err
	}

	err = zbuf.Copy(zw, reader)
	if aerr := reader.Close(); err == nil {
		err = aerr
	}
	return err
}
