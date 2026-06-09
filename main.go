package main

import (
	"fmt"
	"os"

	"github.com/dheeraj-nalapat/lane/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lane:", err)
		os.Exit(1)
	}
}
