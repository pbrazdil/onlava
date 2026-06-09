package main

import (
	"fmt"
	"os"

	"github.com/pbrazdil/onlava/internal/neonselfhost"
)

func main() {
	if err := neonselfhost.Run(os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
