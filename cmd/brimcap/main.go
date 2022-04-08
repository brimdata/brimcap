package main

import (
	"os"

	_ "github.com/brimdata/brimcap/cmd/brimcap/analyze"
	_ "github.com/brimdata/brimcap/cmd/brimcap/config"
	_ "github.com/brimdata/brimcap/cmd/brimcap/cut"
	_ "github.com/brimdata/brimcap/cmd/brimcap/index"
	_ "github.com/brimdata/brimcap/cmd/brimcap/info"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	_ "github.com/brimdata/brimcap/cmd/brimcap/search"
	_ "github.com/brimdata/brimcap/cmd/brimcap/slice"
	_ "github.com/brimdata/brimcap/cmd/brimcap/ts"
)

func main() {
	if err := root.Brimcap.ExecRoot(os.Args[1:]); err != nil {
		root.LogError(err)
		os.Exit(1)
	}
}
