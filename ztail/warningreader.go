package ztail

import (
	"fmt"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zio"
)

type Warner interface {
	Warn(msg string) error
}

type WarningReader struct {
	zr zio.Reader
	wn Warner
}

// NewWarningReader returns a Reader that reads from zr.  Any error
// encountered results in a call to w with the warning as prameter, and
// then a nil *zed.Record and nil error are returned.
func NewWarningReader(zr zio.Reader, w Warner) zio.Reader {
	return &WarningReader{zr: zr, wn: w}
}

func (w *WarningReader) Read() (*zed.Value, error) {
	rec, err := w.zr.Read()
	if err != nil {
		w.wn.Warn(fmt.Sprintf("%s: %s", w.zr, err))
		return nil, nil
	}
	return rec, nil
}
