package cli

import (
	"flag"
	"os"
	"strings"

	"github.com/brimdata/brimcap"
)

type ConfigFlags struct {
	brimcap.Config
}

func (f *ConfigFlags) SetFlags(fs *flag.FlagSet) error {
	if err := f.SetRootOnlyFlags(fs); err != nil {
		return err
	}
	for i := range f.Config.Analyzers {
		f.Config.Analyzers[i].SetFlags(fs)
	}
	return nil
}

func (f *ConfigFlags) SetRootOnlyFlags(fs *flag.FlagSet) error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	// Even though we've already parsed the -config flag, add it to the FlagSet
	// so it appears in help.
	fs.String("config", os.Getenv("BRIMCAP_CONFIG"), "path to config file (env BRIMCAP_CONFIG)")
	defaultRoot := f.Config.RootPath
	if defaultRoot == "" {
		defaultRoot = os.Getenv("BRIMCAP_ROOT")
	}
	fs.StringVar(&f.Config.RootPath, "root", defaultRoot, "path to brimcap root (env BRIMCAP_ROOT)")
	return nil
}

func (f *ConfigFlags) loadConfig() error {
	// Pre-read config file flag so we can determine the rest of the flags in
	// FlagSet.
	path := prereadConfigFlag()
	if path == "" {
		path = os.Getenv("BRIMCAP_CONFIG")
	}
	if path != "" {
		var err error
		f.Config, err = brimcap.LoadConfigYAML(path)
		return err
	}
	f.Config = brimcap.DefaultConfig
	return nil
}

func prereadConfigFlag() string {
	args := os.Args
	for len(args) > 0 {
		s := args[0]
		args = args[1:]
		if s == "-config" {
			return args[0]
		}
		if strings.HasPrefix(s, "-config=") {
			return s[8:]
		}
		continue
	}
	return ""
}
