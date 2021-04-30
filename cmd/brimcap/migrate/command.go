package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/brimdata/brimcap"
	"github.com/brimdata/brimcap/cli"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	"github.com/brimdata/brimcap/pcap"
	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/api/client"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/iosrc"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zbuf"
	"github.com/segmentio/ksuid"
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
	rootflags cli.RootFlags
	zqdroot   string
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
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
	DataURI  iosrc.URI    `json:"data_uri"`
	Name     string       `json:"name"`
	Storage  spaceStorage `json:"storage"`
	TenantID string       `json:"tenant_id"`
}

type pcapMetadata struct {
	PcapURI iosrc.URI
	Span    nano.Span
	Index   pcap.Index
}

func (c *Command) Run(args []string) error {
	if err := c.Command.Init(&c.rootflags); err != nil {
		return err
	}
	defer c.Cleanup()
	if c.zqdroot == "" {
		return errors.New("flag -zqd is required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	c.conn = client.NewConnection()
	if _, err := c.conn.Ping(ctx); err != nil {
		return err
	}
	config, err := c.loadZqdConfig()
	if err != nil {
		return err
	}
	c.logMsg("", fmt.Sprintf("migrating %d spaces", len(config.SpaceRows)))
	for i := range config.SpaceRows {
		if err := c.migrateSpace(ctx, config, i); err != nil && err != errSkip {
			return err
		}
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

func (c *Command) migrateSpace(ctx context.Context, db zqdConfig, idx int) error {
	space := db.SpaceRows[idx]
	path := filepath.Join(c.zqdroot, space.ID)
	m := &migration{
		Command:   c,
		space:     space,
		spaceRoot: path,
	}
	m.logMsg("migration starting")
	if err := m.run(ctx); err != nil {
		if err != errSkip {
			m.logErr(err.Error())
		}
		return err
	}
	m.logMsg("migration successful")
	return nil
}

type log struct {
	Space   string `json:"space,omitempty"`
	Message string `json:"msg,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (c *Command) logMsg(space string, str string) {
	json.NewEncoder(os.Stderr).Encode(log{Space: space, Message: str})
}

func (c *Command) logErr(space string, str string) {
	json.NewEncoder(os.Stderr).Encode(log{Space: space, Error: str})
}

type migration struct {
	*Command
	// brimcapEntry is stored for abort.
	brimcapEntry string
	// poolID is stored for abort.
	poolID    ksuid.KSUID
	space     spaceRow
	spaceRoot string
}

func (c *migration) logMsg(str string) { c.Command.logMsg(c.space.Name, str) }
func (c *migration) logErr(str string) { c.Command.logErr(c.space.Name, str) }

func (m *migration) run(ctx context.Context) error {
	if m.space.Storage.Kind != "filestore" {
		m.logErr(fmt.Sprintf("unsupported storage kind: %s, skipping", m.space.Storage.Kind))
		return errSkip
	}
	pool, err := m.conn.PoolPost(ctx, api.PoolPostRequest{
		Name:  m.space.Name,
		Order: zbuf.OrderDesc,
	})
	if err != nil {
		if errors.Is(err, client.ErrPoolExists) {
			m.logErr("pool already exists with same name, skipping")
			return errSkip
		}
		return err
	}
	m.poolID = pool.ID
	m.logMsg("migrating pcap")
	if err := m.migratePcap(ctx); err != nil {
		m.abort()
		return err
	}
	m.logMsg("migrating data")
	if err := m.migrateData(ctx); err != nil {
		m.abort()
		return err
	}
	m.logMsg("data migration completed")
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
	if _, err = m.conn.LogPostReaders(ctx, m.poolID, nil, f); err != nil {
		return err
	}
	return nil
}

func (m *migration) migratePcap(ctx context.Context) error {
	path := filepath.Join(m.spaceRoot, pcapMetadataFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			m.logMsg("space does not have a pcap")
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
			m.logMsg(fmt.Sprintf("pcapfile %q not found, ignoring pcap", pcappath))
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
	m.conn.PoolDelete(context.Background(), m.poolID)
	os.Remove(m.brimcapEntry)
}
