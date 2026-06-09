package main

import (
	"fmt"
	"os"

	"github.com/dheerajnalapat/lane/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lane:", err)
		os.Exit(1)
	}
}
