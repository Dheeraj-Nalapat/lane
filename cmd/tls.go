package cmd

import (
	"errors"
	"fmt"

	"github.com/dheeraj-nalapat/lane/internal/preflight"
	"github.com/dheeraj-nalapat/lane/internal/proxy"
	"github.com/dheeraj-nalapat/lane/internal/tlsx"
	"github.com/spf13/cobra"
)

var tlsCmd = &cobra.Command{
	Use:   "tls [enable|disable|status]",
	Short: "Manage optional HTTPS for *.localhost (mkcert)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "enable":
			return tlsEnable()
		case "disable":
			return tlsDisable()
		case "status":
			tlsStatus()
			return nil
		default:
			return fmt.Errorf("unknown subcommand %q (use enable|disable|status)", args[0])
		}
	},
}

func init() { root.AddCommand(tlsCmd) }

func tlsEnable() error {
	if err := preflight.DockerRunning(); err != nil {
		return err
	}
	if !tlsx.MkcertInstalled() {
		return errors.New("mkcert is not installed. Install it (e.g. `brew install mkcert`, " +
			"or see https://github.com/FiloSottile/mkcert), then re-run `lane tls enable`")
	}
	if !tlsx.CAPresent() {
		return errors.New("mkcert CA not set up. Run `mkcert -install` once " +
			"(it adds a local CA to your trust store; may prompt for a password), then re-run `lane tls enable`")
	}
	if err := tlsx.Generate(); err != nil {
		return err
	}
	if err := tlsx.WriteTLSConfig(); err != nil {
		return err
	}
	if err := proxy.Up(); err != nil {
		return err
	}
	fmt.Println("HTTPS enabled. Stacks are now reachable on https://<slug>.localhost (and http:// still works).")
	fmt.Println("Re-run `lane up` for any already-running stack to add its HTTPS route.")
	return nil
}

func tlsDisable() error {
	if err := tlsx.Remove(); err != nil {
		return err
	}
	if err := proxy.Up(); err != nil {
		return err
	}
	fmt.Println("HTTPS disabled (proxy back to http only). The mkcert CA is left installed; " +
		"run `mkcert -uninstall` to remove it fully.")
	return nil
}

func tlsStatus() {
	yn := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	fmt.Printf("mkcert installed:   %s\n", yn(tlsx.MkcertInstalled()))
	fmt.Printf("mkcert CA present:  %s\n", yn(tlsx.CAPresent()))
	fmt.Printf("wildcard cert:      %s\n", yn(tlsx.Enabled()))
	fmt.Printf("proxy serving :443: %s\n", yn(tlsx.Enabled() && proxy.Running()))
}
