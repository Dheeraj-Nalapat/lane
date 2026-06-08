package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dheerajnalapat/berth/internal/scaffold"
	"github.com/spf13/cobra"
)

var flagCompose string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a .berth.toml for the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		manifestPath := filepath.Join(dir, ".berth.toml")
		if _, err := os.Stat(manifestPath); err == nil {
			return errors.New(".berth.toml already exists")
		}

		composeRel := flagCompose
		if composeRel == "" {
			for _, c := range []string{"docker-compose.yml", "infra/docker-compose.yml", "compose.yaml"} {
				if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
					composeRel = c
					break
				}
			}
		}
		if composeRel == "" {
			return errors.New("no compose file found; pass --compose <path>")
		}

		body, err := os.ReadFile(filepath.Join(dir, composeRel))
		if err != nil {
			return err
		}
		svc, port := scaffold.GuessWebEntry(string(body))
		if svc == "" {
			return errors.New("could not guess a web entrypoint; edit .berth.toml manually after creation")
		}
		out := scaffold.RenderManifest(filepath.Base(dir), composeRel, svc, port)
		if err := os.WriteFile(manifestPath, []byte(out), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote .berth.toml (routing %s:%d). Review it, then `berth up`.\n", svc, port)
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&flagCompose, "compose", "", "path to the base compose file")
	root.AddCommand(initCmd)
}
