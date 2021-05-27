package migrate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"

	"github.com/brimdata/brimcap"
	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/brimcap/pcap"
	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/api/client"
	"github.com/brimdata/zed/field"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/anyio"
	"github.com/brimdata/zed/zio/zsonio"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
)

var Migrate = &charm.Spec{
	Name:   "migrate",
	Hidden: true,
	Usage:  "migrate [options]",
	Short:  "migrate old zqd spaces to zed lake pools",
	Long: `
Example:

brimcap migrate -zqd=/path/to/zqd -root=/path/to/brimcap/root
`,
	New: New,
}

func init() {
	root.Brimcap.Add(Migrate)
}

type Command struct {
	*root.Command
	conn      *client.Connection
	engine    storage.Engine
	logger    *zap.Logger
	rootflags cli.RootFlags
	zqdroot   string
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	conf := zap.NewProductionConfig()
	conf.Sampling = nil
	logger, err := conf.Build()
	if err != nil {
		return nil, err
	}
	c := &Command{
		Command: parent.(*root.Command),
		logger:  logger,
		engine:  storage.NewLocalEngine(),
	}
	root.LogJSON = true
	f.StringVar(&c.zqdroot, "zqd", "", "path to zqd root")
	c.rootflags.SetFlags(f)
	return c, nil
}

var errSkip = errors.New("skipping this space")

const (
	zqdConfigFile    = "zqd.json"
	pcapMetadataFile = "pcap.json"
	zngFile          = "all.zng"
)

type zqdConfig struct {
	Version   int        `json:"version"`
	SpaceRows []spaceRow `json:"space_rows"`
}

type spaceStorage struct {
	Kind    string          `json:"kind"`
	Archive json.RawMessage `json:"archive,omitempty"`
}

type spaceRow struct {
	ID       string       `json:"id"`
	DataURI  storage.URI  `json:"data_uri"`
	Name     string       `json:"name"`
	Storage  spaceStorage `json:"storage"`
	TenantID string       `json:"tenant_id"`
}

type pcapMetadata struct {
	PcapURI storage.URI
	Span    nano.Span
	Index   pcap.Index
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Command.InitWithContext(&c.rootflags)
	if err != nil {
		return err
	}
	defer cleanup()
	if c.zqdroot == "" {
		return errors.New("flag -zqd is required")
	}
	c.conn = client.NewConnection()
	if _, err := c.conn.Ping(ctx); err != nil {
		return err
	}
	config, err := c.loadZqdConfig()
	if err != nil {
		return err
	}
	c.logger.Info("migrating spaces", zap.Int("count", len(config.SpaceRows)))
	for i := range config.SpaceRows {
		if err := c.migrateSpace(ctx, config, i); err != nil && err != errSkip {
			return err
		}
	}
	config, err = c.loadZqdConfig()
	if err != nil {
		return err
	}
	if len(config.SpaceRows) == 0 {
		c.logger.Info("all spaces migrated, removing old zqd directory")
		return os.RemoveAll(c.zqdroot)
	}
	return nil
}

func (c *Command) loadZqdConfig() (zqdConfig, error) {
	b, err := os.ReadFile(filepath.Join(c.zqdroot, zqdConfigFile))
	if err != nil {
		return zqdConfig{}, err
	}
	var db zqdConfig
	err = json.Unmarshal(b, &db)
	return db, err
}
func unmarshal(r *client.ReadCloser, i interface{}) error {
	format, err := api.MediaTypeToFormat(r.ContentType)
	if err != nil {
		return err
	}
	zr, err := anyio.NewReaderWithOpts(r, zson.NewContext(), anyio.ReaderOpts{Format: format})
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

func (c *Command) migrateSpace(ctx context.Context, db zqdConfig, idx int) error {
	space := db.SpaceRows[idx]
	path := filepath.Join(c.zqdroot, space.ID)
	logger := c.logger.With(zap.String("space", space.Name))
	if space.Storage.Kind != "filestore" {
		logger.Warn("unsupported storage kind, skipping", zap.String("kind", space.Storage.Kind))
		return errSkip
	}
	r, err := c.conn.PoolPost(ctx, api.PoolPostRequest{
		Name: space.Name,
		Layout: order.Layout{
			Order: order.Desc,
			Keys:  field.List{field.Dotted("ts")},
		},
	})
	if err != nil {
		if errors.Is(err, client.ErrPoolExists) {
			logger.Warn("pool already exists with same name, skipping")
			return errSkip
		}
		return err
	}
	var pool lake.PoolConfig
	if err := unmarshal(r, &pool); err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	m := &migration{
		Command:   c,
		logger:    logger.With(zap.String("pool_id", pool.ID.String())),
		poolID:    pool.ID,
		space:     space,
		spaceRoot: path,
	}
	m.logger.Info("migration starting")
	if err := m.run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			m.logger.Warn("migration aborted")
		} else if !errors.Is(err, errSkip) {
			m.logger.Error("migration error", zap.Error(err))
		}
		return err
	}
	m.logger.Info("migration successful")
	return nil
}

type migration struct {
	*Command
	logger *zap.Logger
	// brimcapEntry is stored for abort.
	brimcapEntry string
	// poolID is stored for abort.
	poolID    ksuid.KSUID
	space     spaceRow
	spaceRoot string
}

func (m *migration) run(ctx context.Context) error {
	m.logger.Info("migrating pcap")
	if err := m.migratePcap(ctx); err != nil {
		m.abort()
		return err
	}
	m.logger.Info("migrating data")
	if err := m.migrateData(ctx); err != nil {
		m.abort()
		return err
	}
	m.logger.Info("data migration completed")
	return m.removeSpace()
}

func (m *migration) migrateData(ctx context.Context) error {
	zngpath := filepath.Join(m.spaceRoot, zngFile)
	f, err := os.Open(zngpath)
	if err != nil {
		if os.IsNotExist(err) {

			return errSkip
		}
		return err
	}
	defer f.Close()
	if _, err = m.conn.LogPostReaders(ctx, m.engine, m.poolID, nil, f); err != nil {
		return err
	}
	return nil
}

func (m *migration) migratePcap(ctx context.Context) error {
	path := filepath.Join(m.spaceRoot, pcapMetadataFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			m.logger.Info("space does not have a pcap")
			return nil
		}
		return err
	}
	var meta pcapMetadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return err
	}
	pcappath := meta.PcapURI.Filepath()
	f, err := os.Open(pcappath)
	if err != nil {
		if os.IsNotExist(err) {
			m.logger.Info("pcapfile not found, ignoring pcap", zap.String("pcap_path", pcappath))
			return nil
		}
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return err
	}
	b, err = json.Marshal(brimcap.File{PcapPath: pcappath, Index: meta.Index})
	if err != nil {
		return err
	}
	rootentry := m.rootflags.Root.Filepath(hash)
	info, _ := os.Stat(rootentry)
	if info == nil {
		// Only write if hash doesn't exist in brimcap root.
		// Store brimcap entry path in case we need to issue an abort.
		m.brimcapEntry = rootentry
		return os.WriteFile(rootentry, b, 0600)
	}
	return nil
}

// removeSpace removes the space from the config file and deletes the space's
// data directory.
func (m *migration) removeSpace() error {
	config, err := m.loadZqdConfig()
	if err != nil {
		return err
	}
	spaces := config.SpaceRows
	for i, space := range config.SpaceRows {
		if space.ID == m.space.ID {
			spaces = append(spaces[:i], spaces[i+1:]...)
		}
	}
	config.SpaceRows = spaces
	b, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(m.zqdroot, zqdConfigFile), b, 0600); err != nil {
		return err
	}
	return os.RemoveAll(m.spaceRoot)
}

func (m *migration) abort() {
	if err := os.Remove(m.brimcapEntry); err != nil {
		m.logger.Error("error removing brimcap entry from aborted migration", zap.Error(err))
	}
}
