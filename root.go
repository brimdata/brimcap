package brimcap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/brimdata/brimcap/pcap"
	"github.com/brimdata/brimcap/pcap/pcapio"
	pkgfs "github.com/brimdata/zed/pkg/fs"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zbuf"
	"golang.org/x/sync/errgroup"
)

type Search struct {
	Span    nano.Span
	Proto   string
	SrcIP   net.IP
	SrcPort uint16
	DstIP   net.IP
	DstPort uint16
}

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

	return index.Span(), r.AddPcapWithIndex(path, index)
}

func (r Root) AddPcapWithIndex(path string, index pcap.Index) (err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	// symlink path to root
	if err := os.Symlink(path, r.SymlinkPath(path)); err != nil {
		return err
	}

	// create pcap index
	if err := pkgfs.MarshalJSONFile(index, r.IndexPath(path), 0644); err != nil {
		r.DeletePcap(path)
		return err
	}
	return nil
}

func (r Root) Search(ctx context.Context, req Search, w io.Writer) error {
	var search pcap.Search
	// We add two microseconds to the end of the span as fudge to deal with the
	// fact that zeek truncates timestamps to microseconds where pcap-ng
	// timestamps have nanosecond precision.  We need two microseconds because
	// both the base timestamp of a conn record as well as the duration time
	// can be truncated downward.
	span := nano.NewSpanTs(req.Span.Ts, req.Span.End()+2000)
	flow := pcap.NewFlow(req.SrcIP, int(req.SrcPort), req.DstIP, int(req.DstPort))
	switch req.Proto {
	case "tcp":
		search = pcap.NewTCPSearch(span, flow)
	case "udp":
		search = pcap.NewUDPSearch(span, flow)
	case "icmp":
		search = pcap.NewICMPSearch(span, req.SrcIP, req.DstIP)
	default:
		return fmt.Errorf("unsupported proto type: %s", req.Proto)
	}

	files, err := r.Pcaps()
	if err != nil {
		return err
	}

	group, ctx := errgroup.WithContext(ctx)
	readers := make(chan *pcap.SearchReader, len(files))
	closers := make(chan io.Closer, len(files))
	for _, file := range files {
		file := file
		group.Go(func() error {
			pr, closer, err := file.PcapReader(span)
			if err != nil || pr == nil {
				return err
			}

			r, err := search.Reader(ctx, pr)
			if err != nil {
				closer.Close()
				if errors.Is(err, pcap.ErrNoPcapsFound) {
					return nil
				}
				return err
			}

			readers <- r
			closers <- closer
			return nil
		})
	}
	go func() {
		group.Wait()
		close(readers)
		close(closers)
	}()

	defer func() {
		for closer := range closers {
			closer.Close()
		}
	}()

	var count int
	for pr := range readers {
		count++
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := io.Copy(w, pr); err != nil {
			return err
		}
	}
	err = group.Wait()
	if err == nil && count == 0 {
		return pcap.ErrNoPcapsFound
	}
	return err
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

type File struct {
	LinkPath  string
	IndexPath string
}

func (f File) PcapReader(span nano.Span) (pcapio.Reader, io.Closer, error) {
	index, err := pcap.LoadIndex(f.IndexPath)
	if err != nil {
		return nil, nil, err
	}

	file, err := os.Open(f.LinkPath)
	if err != nil {
		return nil, nil, err
	}

	slicer, err := pcap.NewSlicer(file, index, span)
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if slicer == nil {
		file.Close()
		return nil, nil, nil
	}

	pcapReader, err := pcapio.NewReader(slicer)
	if err != nil {
		file.Close()
		return nil, nil, err
	}

	return pcapReader, file, nil
}

func (r Root) Pcaps() ([]File, error) {
	entries, err := os.ReadDir(string(r))
	if err != nil {
		return nil, err
	}
	var files []File
	for _, entry := range entries {
		if entry.Type()&fs.ModeSymlink != 0 {
			files = append(files, File{
				LinkPath:  r.SymlinkPath(entry.Name()),
				IndexPath: r.IndexPath(entry.Name()),
			})
		}
	}
	return files, nil
}

func (r Root) join(els ...string) string {
	return filepath.Join(append([]string{string(r)}, els...)...)
}
