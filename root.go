package brimcap

import (
	"os"
	"path/filepath"

	"github.com/brimdata/brimcap/pcap"
	"github.com/brimdata/zed/pkg/fs"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zbuf"
)

type Root string

// AddPcap adds the pcap path to the BRIMCAP_ROOT, meaning a symlink for the pcap is
// added to the root directory along with an index of the pcap.
func (r Root) AddPcap(path string, limit int, warner zbuf.Warner) (nano.Span, error) {
	f, err := os.Open(path)
	if err != nil {
		return nano.Span{}, nil
	}
	defer f.Close()

	index, err := pcap.CreateIndexWithWarnings(f, limit, warner)
	if err != nil {
		return nano.Span{}, err
	}

	// symlink path to root
	if err := os.Symlink(path, r.SymlinkPath(path)); err != nil {
		return nano.Span{}, err
	}

	// create pcap index
	if err = fs.MarshalJSONFile(index, r.IndexPath(path), 0644); err != nil {
		r.DeletePcap(path)
		return nano.Span{}, err
	}

	return index.Span(), nil
}

func (r Root) SymlinkPath(path string) string {
	return r.join(filepath.Base(path))
}

func (r Root) IndexPath(path string) string {
	return r.join(filepath.Base(path) + ".idx")
}

// DeletePcap removes all files associated with the pcap path (if they exist).
func (r Root) DeletePcap(path string) error {
	err1 := os.Remove(r.SymlinkPath(path))
	err2 := os.Remove(r.IndexPath(path))
	if err1 != nil && !os.IsNotExist(err1) {
		return err1
	}
	if os.IsNotExist(err2) {
		err2 = nil
	}
	return err2
}

func (r Root) join(els ...string) string {
	return filepath.Join(append([]string{string(r)}, els...)...)
}
