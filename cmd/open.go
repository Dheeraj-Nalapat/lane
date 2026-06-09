package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/dheerajnalapat/lane/internal/dockerx"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open a stack's URL in the browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		stacks, err := dockerx.List()
		if err != nil {
			return err
		}
		var url string
		for _, s := range stacks {
			if (flagSlug == "" || s.Slug == flagSlug) && s.URL != "" {
				url = s.URL
				break
			}
		}
		if url == "" {
			return fmt.Errorf("no running stack with a URL (use --slug)")
		}
		opener := "xdg-open"
		if runtime.GOOS == "darwin" {
			opener = "open"
		}
		return exec.Command(opener, url).Start()
	},
}

func init() { root.AddCommand(openCmd) }
