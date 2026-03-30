package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	root := newRootCommand()
	root.Version = version
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
