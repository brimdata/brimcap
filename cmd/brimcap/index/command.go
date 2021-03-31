package index

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/brimcap/pcap"
	"github.com/brimdata/zed/pkg/charm"
)

var Index = &charm.Spec{
	Name:  "index",
	Usage: "index [options]",
	Short: "creates time index files for pcaps for use by pcap slice",
	Long: `
The index command creates a time index for a pcap file.  The pcap file is not
modified or copied.

Roughly speaking, the index is a list of slots that represents
a seek offset and time range covered by the packets starting at the offset
and ending at the seek offset specified in the next slot.  It also includes
offset information for section and interface headers for pcap-ng format so
all blocks with referenced metadata are included in the output pcap.

The number of index slots is bounded by -n argument (technically speaking,
the number of slots is computed by choosing D, the smallest
power-of-2 divisor of N, the number of packets in the pcap file, such that N / D
is less than or equal to the limit specified by -n).

The output is written in json format to standard output or if -x is specified,
to the indicate file.
`,
	New: New,
}

func init() {
	root.Brimcap.Add(Index)
}

type Command struct {
	*root.Command
	limit      int
	inputFile  string
	outputFile string
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	f.StringVar(&c.inputFile, "r", "-", "input file to read from or stdin if -")
	f.StringVar(&c.outputFile, "x", "-", "name of output file for the index or - for stdout")
	f.IntVar(&c.limit, "n", 10000, "limit on index size")
	return c, nil
}

func (c *Command) Run(args []string) (err error) {
	defer c.Cleanup()
	if err := c.Init(); err != nil {
		return err
	}

	f := os.Stdin
	if c.inputFile != "-" {
		f, err = os.Open(c.inputFile)
		if err != nil {
			return err
		}
		defer f.Close()
	}

	warn := make(chan string)
	var index pcap.Index
	go func() {
		index, err = pcap.CreateIndexWithWarnings(f, c.limit, warn)
		close(warn)
	}()
	for s := range warn {
		fmt.Fprintf(os.Stderr, "warning: %s\n", s)
	}
	if err != nil {
		return err
	}
	b, err := json.Marshal(index)
	if err != nil {
		return err
	}
	if c.outputFile == "-" {
		fmt.Println(string(b))
		return nil
	}
	return os.WriteFile(c.outputFile, b, 0644)
}
