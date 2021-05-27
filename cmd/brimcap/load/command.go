package analyze

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cli/analyzecli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/api/client"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/anyio"
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

func (c *Command) Run(args []string) error {
	if len(args) != 1 {
		return errors.New("expected 1 pcapfile arg")
	} else if args[0] == "-" {
		return errors.New("reading a pcap from stdin not supported")
	}
	ctx, cleanup, err := c.Command.InitWithContext(&c.analyzeflags, &c.rootflags)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := c.lookupPool(ctx); err != nil {
		return err
	}
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
	zctx := zson.NewContext()
	analyzer := analyzer.CombinerWithContext(ctx, zctx, pcapfile, c.analyzeflags.Configs...)
	go display.Run(analyzer, stat.Size(), span)
	reader := toioreader(analyzer)
	_, err = c.conn.LogPostReaders(ctx, storage.NewLocalEngine(), c.poolID, nil, reader)
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

func (c *Command) lookupPool(ctx context.Context) error {
	if c.poolName == "" {
		return errors.New("pool (-p) must be specified")
	}
	c.conn = client.NewConnection()
	r, err := c.conn.PoolScan(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	format, err := api.MediaTypeToFormat(r.ContentType)
	if err != nil {
		return err
	}
	zr, err := anyio.NewReaderWithOpts(r, zson.NewContext(), anyio.ReaderOpts{Format: format})
	if err != nil {
		return nil
	}
	for {
		rec, err := zr.Read()
		if rec == nil {
			break
		}
		if err != nil {
			return err
		}
		var pool lake.PoolConfig
		if err := zson.UnmarshalZNGRecord(rec, &pool); err != nil {
			return err
		}
		if pool.Name == c.poolName {
			c.poolID = pool.ID
			return nil
		}
	}
	return fmt.Errorf("pool %q not found", c.poolName)
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
