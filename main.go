package main

import (
	"fmt"
	"os"

	"github.com/dheerajnalapat/berth/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "berth:", err)
		os.Exit(1)
	}
}
