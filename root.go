package brimcap

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/brimdata/brimcap/pcap"
	"github.com/brimdata/brimcap/pcap/pcapio"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zio"
	"golang.org/x/sync/errgroup"
)

const indexPrefix = "idx-"

type Search struct {
	Span    nano.Span
	Proto   string
	SrcIP   net.IP
	SrcPort uint16
	DstIP   net.IP
	DstPort uint16
}

type Root string

// AddPcap adds the pcap path to the brimcap root.
func (r Root) AddPcap(pcappath string, limit int, warner zio.Warner) (nano.Span, error) {
	f, err := os.Open(pcappath)
	if err != nil {
		return nano.Span{}, err
	}
	defer f.Close()
	if pcappath, err = filepath.Abs(pcappath); err != nil {
		return nano.Span{}, err
	}
	hash := sha256.New()
	reader := io.TeeReader(f, hash)
	index, err := pcap.CreateIndexWithWarnings(reader, limit, warner)
	if err != nil {
		return nano.Span{}, err
	}
	b, err := json.Marshal(File{PcapPath: filepath.Clean(pcappath), Index: index})
	if err != nil {
		return nano.Span{}, err
	}
	return index.Span(), os.WriteFile(r.Filepath(hash), b, 0600)
}

func (r Root) Filepath(hash hash.Hash) string {
	name := indexPrefix + base64.RawURLEncoding.EncodeToString(hash.Sum(nil)) + ".json"
	return r.join(name)
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

// DeletePcap removes all files associated with the pcap path (if they exist).
func (r Root) DeletePcap(pcappath string) (err error) {
	pcappath, err = filepath.Abs(pcappath)
	if err != nil {
		return err
	}
	files, err := r.Pcaps()
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.PcapPath == pcappath {
			if err := os.Remove(file.path); err != nil {
				return err
			}
		}
	}
	return nil
}

type File struct {
	Index    pcap.Index `json:"index"`
	PcapPath string     `json:"pcap_path"`

	path string
}

func (f File) PcapReader(span nano.Span) (pcapio.Reader, io.Closer, error) {
	file, err := os.Open(f.PcapPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			os.Remove(f.path)
			err = nil
		}
		return nil, nil, err
	}

	slicer, err := pcap.NewSlicer(file, f.Index, span)
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
		if strings.HasPrefix(entry.Name(), indexPrefix) {
			path := r.join(entry.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}

			file := File{path: path}
			if err := json.Unmarshal(b, &file); err != nil {
				return nil, err
			}

			files = append(files, file)
		}
	}
	return files, nil
}

func (r Root) join(els ...string) string {
	return filepath.Join(append([]string{string(r)}, els...)...)
}
