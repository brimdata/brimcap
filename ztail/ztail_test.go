package ztail

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/runtime"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/anyio"
	"github.com/brimdata/zed/zio/zsonio"
	"github.com/stretchr/testify/suite"
)

var sortTs = compiler.MustParse("sort ts")

const expected = `{ts:1970-01-01T00:00:00Z}
{ts:1970-01-01T00:00:01Z}
{ts:1970-01-01T00:00:02Z}
{ts:1970-01-01T00:00:03Z}
{ts:1970-01-01T00:00:04Z}
{ts:1970-01-01T00:00:05Z}
{ts:1970-01-01T00:00:06Z}
{ts:1970-01-01T00:00:07Z}
{ts:1970-01-01T00:00:08Z}
{ts:1970-01-01T00:00:09Z}
{ts:1970-01-01T00:00:10Z}
{ts:1970-01-01T00:00:11Z}
{ts:1970-01-01T00:00:12Z}
{ts:1970-01-01T00:00:13Z}
{ts:1970-01-01T00:00:14Z}
{ts:1970-01-01T00:00:15Z}
{ts:1970-01-01T00:00:16Z}
{ts:1970-01-01T00:00:17Z}
{ts:1970-01-01T00:00:18Z}
{ts:1970-01-01T00:00:19Z}
`

type tailerTSuite struct {
	suite.Suite
	dir  string
	zctx *zed.Context
	dr   *Tailer
}

func TestTailer(t *testing.T) {
	suite.Run(t, new(tailerTSuite))
}

func (s *tailerTSuite) SetupTest() {
	s.dir = s.T().TempDir()
	s.zctx = zed.NewContext()
	var err error
	s.dr, err = New(s.zctx, s.dir, anyio.ReaderOpts{Format: "zson"}, nil)
	s.Require().NoError(err)
}

func (s *tailerTSuite) TestCreatedFiles() {
	result, errCh := s.read()
	f1 := s.createFile("test1.zson")
	f2 := s.createFile("test2.zson")
	s.write(f1, f2)
	s.Require().NoError(<-errCh)
	s.Equal(expected, <-result)
}

func (s *tailerTSuite) TestIgnoreDir() {
	result, errCh := s.read()
	f1 := s.createFile("test1.zson")
	f2 := s.createFile("test2.zson")
	err := os.Mkdir(filepath.Join(s.dir, "testdir"), 0755)
	s.Require().NoError(err)
	s.write(f1, f2)
	s.Require().NoError(<-errCh)
	s.Equal(expected, <-result)
}

func (s *tailerTSuite) TestExistingFiles() {
	f1 := s.createFile("test1.zson")
	f2 := s.createFile("test2.zson")
	result, errCh := s.read()
	s.write(f1, f2)
	s.Require().NoError(<-errCh)
	s.Equal(expected, <-result)
}

func (s *tailerTSuite) TestEmptyFile() {
	result, errCh := s.read()
	f1 := s.createFile("test1.zson")
	_ = s.createFile("test2.zson")
	s.write(f1)
	s.Require().NoError(<-errCh)
	s.Equal(expected, <-result)
}

func (s *tailerTSuite) createFile(name string) *os.File {
	f, err := os.Create(filepath.Join(s.dir, name))
	s.Require().NoError(err)
	s.T().Cleanup(func() { f.Close() })
	// Call sync to ensure fs events are sent in a timely matter.
	s.Require().NoError(f.Sync())
	return f
}

func (s *tailerTSuite) read() (<-chan string, <-chan error) {
	result := make(chan string)
	errCh := make(chan error)
	buf := bytes.NewBuffer(nil)
	w := zsonio.NewWriter(zio.NopCloser(buf), zsonio.WriterOpts{})
	go func() {
		comp := compiler.NewCompiler()
		query, err := runtime.CompileQuery(context.Background(), s.zctx, comp, sortTs, nil, []zio.Reader{s.dr})
		if err != nil {
			close(result)
			errCh <- err
			return
		}
		if err = zio.Copy(w, runtime.AsReader(query)); err != nil {
			close(result)
			errCh <- err
		} else {
			close(errCh)
			result <- buf.String()
		}
	}()
	return result, errCh
}

func (s *tailerTSuite) write(files ...*os.File) {
	lines := strings.Split(strings.TrimSpace(expected), "\n")
loop:
	for i := 0; ; {
		for _, f := range files {
			_, err := f.WriteString(lines[i])
			s.Require().NoError(err)
			if i += 1; i >= len(lines) {
				break loop
			}
		}
	}
	// Need to sync here as on windows the fsnotify event is not triggered
	// unless this is done. Presumably this happens in cases when not enough
	// data has been written so the system has not flushed the file buffer to disk.
	for _, f := range files {
		s.Require().NoError(f.Sync())
	}
	s.Require().NoError(s.dr.Stop())
}
