package main

import (
	"fmt"
	"os"

	_ "net/http/pprof"

	_ "github.com/brimsec/brimcap/cmd/brimcap/analyze"
	_ "github.com/brimsec/brimcap/cmd/brimcap/cut"
	_ "github.com/brimsec/brimcap/cmd/brimcap/index"
	_ "github.com/brimsec/brimcap/cmd/brimcap/info"
	_ "github.com/brimsec/brimcap/cmd/brimcap/load"
	"github.com/brimsec/brimcap/cmd/brimcap/root"
	_ "github.com/brimsec/brimcap/cmd/brimcap/slice"
	_ "github.com/brimsec/brimcap/cmd/brimcap/ts"
)

// Version is set via the Go linker.
var version = "unknown"

func main() {
	//XXX
	//root.Version = version
	if _, err := root.Brimcap.ExecRoot(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
