package cli

import (
	"errors"
	"flag"
	"os"

	"github.com/brimdata/brimcap"
)

type RootFlags struct {
	root     string
	Optional bool
	IsSet    bool
	Root     brimcap.Root
}

func (f *RootFlags) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&f.root, "root", "", "location of brimcap root (where indices are saved)")
}

func (f *RootFlags) Init() error {
	if f.root == "" {
		if !f.Optional {
			return errors.New("brimcap root (-root) must be specified")
		}
		return nil
	}
	f.IsSet = true

	info, err := os.Stat(f.root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("brimcap root must be a directory")
	}
	f.Root = brimcap.Root(f.root)
	return nil
}
