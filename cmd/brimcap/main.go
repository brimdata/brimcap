package main

import (
	"os"

	_ "github.com/brimdata/brimcap/cmd/brimcap/analyze"
	_ "github.com/brimdata/brimcap/cmd/brimcap/cut"
	_ "github.com/brimdata/brimcap/cmd/brimcap/index"
	_ "github.com/brimdata/brimcap/cmd/brimcap/info"
	_ "github.com/brimdata/brimcap/cmd/brimcap/launch"
	_ "github.com/brimdata/brimcap/cmd/brimcap/load"
	"github.com/brimdata/brimcap/cmd/brimcap/root"
	_ "github.com/brimdata/brimcap/cmd/brimcap/search"
	_ "github.com/brimdata/brimcap/cmd/brimcap/slice"
	_ "github.com/brimdata/brimcap/cmd/brimcap/ts"
)

func main() {
	if root.Brimcap.ExecRoot(os.Args[1:]) != nil {
		os.Exit(1)
	}
}
