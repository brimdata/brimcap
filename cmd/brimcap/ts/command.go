package slice

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/brimcap/pcap/pcapio"
	"github.com/brimdata/zed/pkg/charm"
)

var Ts = &charm.Spec{
	Name:  "ts",
	Usage: "ts [options] ts",
	Short: "print timestamps of a pcap",
	Long: `
The ts command prints the time stamps of each packet in the input pcap in
fractional seconds.  This is useful for testing.
`,
	New: New,
}

func init() {
	root.Brimcap.Add(Ts)
}

type Command struct {
	inputFile  string
	outputFile string
	*root.Command
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.Command.Child = c
	f.StringVar(&c.inputFile, "r", "-", "file to read from or stdin if -")
	f.StringVar(&c.outputFile, "w", "-", "file to write to or stdout if -")
	return c, nil
}

func (c *Command) Exec(args []string) error {
	defer c.Cleanup()
	if err := c.Init(); err != nil {
		return err
	}
	if len(args) != 0 {
		return errors.New("pcap ts takes no arguments")
	}
	in := os.Stdin
	if c.inputFile != "-" {
		var err error
		in, err = os.Open(c.inputFile)
		if err != nil {
			return err
		}
		defer in.Close()
	}
	reader, err := pcapio.NewReader(in)
	if err != nil {
		return err
	}
	out := os.Stdout
	if c.outputFile != "-" {
		var err error
		out, err = os.OpenFile(c.outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer out.Close()
	}
	for {
		block, typ, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if block == nil {
			break
		}
		if typ == pcapio.TypePacket {
			_, ts, _, err := reader.Packet(block)
			if err != nil {
				return err
			}
			fmt.Fprintln(out, ts.StringFloat())
		}
	}
	return nil
}
