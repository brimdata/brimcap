package analyze

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/brimdata/brimcap/analyzer"
	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cli/analyzecli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/api/client"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/anyio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zio/zsonio"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
	"golang.org/x/sync/errgroup"
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
	analyzecli.Display
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
	pcappath := args[0]
	span, err := c.rootflags.Root.AddPcap(pcappath, c.limit, c)
	if err != nil {
		return fmt.Errorf("error writing brimcap root: %w", err)
	}
	abort := func() { c.rootflags.Root.DeletePcap(pcappath) }
	pcapfile, err := cli.OpenFileArg(pcappath)
	if err != nil {
		abort()
		return err
	}
	defer pcapfile.Close()
	info, err := pcapfile.Stat()
	if err != nil {
		abort()
		return err
	}
	c.Display = analyzecli.NewDisplay(root.LogJSON, info.Size(), span)
	defer c.Display.End()
	pr, pw := io.Pipe()
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		zw := zngio.NewWriter(pw, zngio.WriterOpts{})
		return analyzer.Run(ctx, pcapfile, zw, c, time.Second, c.analyzeflags.Configs...)
	})
	group.Go(func() error {
		return c.post(ctx, pr)
	})
	if err := group.Wait(); err != nil {
		abort()
		return err
	}
	return nil
}

func (c *Command) post(ctx context.Context, pr *io.PipeReader) error {
	res, err := c.conn.Add(ctx, c.poolID, pr)
	if err != nil {
		return err
	}
	var add api.AddResponse
	err = unmarshal(res, &add)
	res.Body.Close()
	if err != nil {
		return err
	}
	return c.conn.Commit(ctx, c.poolID, add.Commit, api.CommitRequest{})
}

func unmarshal(r *client.Response, i interface{}) error {
	format, err := api.MediaTypeToFormat(r.ContentType)
	if err != nil {
		return err
	}
	zr, err := anyio.NewReaderWithOpts(r.Body, zson.NewContext(), anyio.ReaderOpts{Format: format})
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	zw := zsonio.NewWriter(zio.NopCloser(&buf), zsonio.WriterOpts{})
	if err := zio.Copy(zw, zr); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return zson.Unmarshal(buf.String(), i)
}

func (c *Command) lookupPool(ctx context.Context) error {
	if c.poolName == "" {
		return errors.New("pool (-p) must be specified")
	}
	c.conn = client.NewConnection()
	r, err := c.conn.ScanPools(ctx)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	format, err := api.MediaTypeToFormat(r.ContentType)
	if err != nil {
		return err
	}
	zr, err := anyio.NewReaderWithOpts(r.Body, zson.NewContext(), anyio.ReaderOpts{Format: format})
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
