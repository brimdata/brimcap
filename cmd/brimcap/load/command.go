package analyze

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cli/analyzecli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/zed/api/client"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/signalctx"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
)

var Load = &charm.Spec{
	Name:  "load",
	Usage: "load [options] pcap",
	Short: "analyze a pcap and send logs into the Brim desktop client",
	Long: `
The load command is the same as the analyze command except the output stream of
generated logs is written to a specified pool in the Brim desktop client.

brimcap load -p mypool file.pcap
`,
	New: New,
}

func init() {
	root.Brimcap.Add(Load)
}

type Command struct {
	*root.Command
	analyzeflags analyzecli.Flags
	conn         *client.Connection
	limit        int
	rootflags    cli.RootFlags
	poolName     string
	poolID       ksuid.KSUID
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.analyzeflags.SetFlags(f)
	c.rootflags.SetFlags(f)
	f.StringVar(&c.poolName, "p", "", "name of Zed lake pool")
	f.IntVar(&c.limit, "n", 10000, "limit in bytes on index size")
	return c, nil
}

func (c *Command) Init() error {
	if c.poolName == "" {
		return errors.New("pool (-p) must be specified")
	}

	c.conn = client.NewConnection()
	list, err := c.conn.PoolList(context.TODO())
	if err != nil {
		return err
	}
	for _, pool := range list {
		if pool.Name == c.poolName {
			c.poolID = pool.ID
			return nil
		}
	}
	return fmt.Errorf("pool %q not found", c.poolName)
}

func (c *Command) Run(args []string) (err error) {
	if len(args) != 1 {
		return errors.New("expected 1 pcapfile arg")
	} else if args[0] == "-" {
		return errors.New("reading a pcap from stdin not supported")
	}

	if err := c.Command.Init(c, &c.analyzeflags, &c.rootflags); err != nil {
		return err
	}
	defer c.Cleanup()

	if err := c.AddRunnersToPath(); err != nil {
		return err
	}

	display := analyzecli.NewDisplay(root.LogJSON)
	pcappath := args[0]
	root := c.rootflags.Root

	span, err := root.AddPcap(pcappath, c.limit, display)
	if err != nil {
		return fmt.Errorf("error writing brimcap root: %w", err)
	}

	pcapfile, err := cli.OpenFileArg(pcappath)
	if err != nil {
		root.DeletePcap(pcappath)
		return err
	}
	defer pcapfile.Close()

	stat, err := pcapfile.Stat()
	if err != nil {
		root.DeletePcap(pcappath)
		return err
	}

	ctx, cancel := signalctx.New(os.Interrupt)
	defer cancel()

	zctx := zson.NewContext()
	analyzer := analyzer.CombinerWithContext(ctx, zctx, pcapfile, c.analyzeflags.Configs...)
	go display.Run(analyzer, stat.Size(), span)

	reader := toioreader(analyzer)
	_, err = c.conn.LogPostReaders(ctx, c.poolID, nil, reader)
	reader.Close()

	if aerr := analyzer.Close(); err == nil {
		err = aerr
	}
	if err != nil {
		root.DeletePcap(pcappath)
	}
	display.Close()
	return err
}

type ioreader struct {
	reader io.Reader
	writer *io.PipeWriter
	closer io.Closer
}

// toioreader transforms a zio.Reader into an io.Reader that emits zng bytes.
func toioreader(r zio.Reader) io.ReadCloser {
	pr, pw := io.Pipe()
	i := &ioreader{reader: pr, writer: pw}
	w := zngio.NewWriter(pw, zngio.WriterOpts{})
	go i.run(r, w)
	return i
}

func (i *ioreader) run(r zio.Reader, w zio.Writer) {
	err := zio.Copy(w, r)
	if err != nil {
		i.writer.CloseWithError(err)
	}
	i.writer.Close()
}

func (i *ioreader) Read(b []byte) (int, error) {
	return i.reader.Read(b)
}

func (i *ioreader) Close() error {
	return i.writer.Close()
}
