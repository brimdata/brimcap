package analyze

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/brimsec/brimcap/analyzer"
	"github.com/brimsec/brimcap/cmd/brimcap/root"
	"github.com/brimsec/zq/api"
	"github.com/brimsec/zq/api/client"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio/zngio"
	"github.com/mccanne/charm"
)

var Load = &charm.Spec{
	Name:  "load",
	Usage: "load [options] [ pcapfile ]",
	Short: "",
	Long:  ``,
	New:   New,
}

func init() {
	root.Brimcap.Add(Load)
}

type Command struct {
	*root.Command
	analyze analyzer.Flags
	conn    *client.Connection
	space   string
	spaceID api.SpaceID
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.analyze.SetFlags(f)
	f.StringVar(&c.space, "s", "", "name of zqd space")
	return c, nil
}

func (c *Command) Init() error {
	if c.space == "" {
		return errors.New("a space must be specified")
	}
	c.conn = client.NewConnection()
	list, err := c.conn.SpaceList(context.TODO())
	if err != nil {
		return err
	}
	for _, sp := range list {
		if sp.Name == c.space {
			c.spaceID = sp.ID
			return nil
		}
	}
	return fmt.Errorf("space %q not found", c.space)
}

func (c *Command) Run(args []string) (err error) {
	if len(args) != 1 {
		return errors.New("expected 1 pcapfile arg")
	}

	if err := c.Command.Init(c, &c.analyze); err != nil {
		return err
	}
	defer c.Cleanup()

	pcapfile := os.Stdin
	if args[0] != "-" {
		if pcapfile, err = os.Open(args[0]); err != nil {
			return fmt.Errorf("error loading pcap file: %w", err)
		}
	}
	defer pcapfile.Close()

	zr, err := c.analyze.Open(pcapfile)
	if err != nil {
		return err
	}
	defer zr.Close()

	reader := toioreader(zr)
	_, err = c.conn.LogPostReaders(context.TODO(), c.spaceID, nil, reader)
	reader.Close()
	return err
}

type ioreader struct {
	reader io.Reader
	writer *io.PipeWriter
}

// toioreader transforms a zbuf.Reader into an io.Reader that emits zng bytes.
func toioreader(r zbuf.Reader) io.ReadCloser {
	pr, pw := io.Pipe()
	i := &ioreader{reader: pr, writer: pw}
	w := zngio.NewWriter(pw, zngio.WriterOpts{})
	go i.run(r, w)
	return i
}

func (i *ioreader) run(r zbuf.Reader, w zbuf.Writer) {
	err := zbuf.Copy(w, r)
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
