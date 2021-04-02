package cli

import (
	"errors"
	"flag"
	"os"
)

type RootFlags struct {
	Root string
}

func (f *RootFlags) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&f.Root, "root", "", "location of brimcap root (where indices are saved)")
}

func (f *RootFlags) Init() error {
	if f.Root == "" {
		return errors.New("brimcap root (-root) must be specified")
	}

	info, err := os.Stat(f.Root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("brimcap root must be a directory")
	}
	return nil
}
