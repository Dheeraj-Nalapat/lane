// Package proxy manages the shared Traefik container and the lane network.
package proxy

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/dheerajnalapat/lane/internal/paths"
)

//go:embed traefik-compose.yml.tmpl
var composeTmpl string

// Network is the shared external Docker network name.
const Network = "lane"

func renderCompose(network, dynamicDir string) ([]byte, error) {
	t, err := template.New("c").Parse(composeTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]string{"Network": network, "DynamicDir": dynamicDir})
	return buf.Bytes(), err
}

func composePath() string { return filepath.Join(paths.Traefik(), "docker-compose.yml") }

// ensureNetwork creates the external network if it does not exist.
func ensureNetwork() error {
	// `docker network inspect` exits non-zero when missing.
	if exec.Command("docker", "network", "inspect", Network).Run() == nil {
		return nil
	}
	out, err := exec.Command("docker", "network", "create", Network).CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating %s network: %v\n%s", Network, err, out)
	}
	return nil
}

// Up ensures paths, network, the rendered compose file, and starts Traefik.
func Up() error {
	if err := paths.Ensure(); err != nil {
		return err
	}
	if err := ensureNetwork(); err != nil {
		return err
	}
	body, err := renderCompose(Network, paths.TraefikDynamic())
	if err != nil {
		return err
	}
	if err := os.WriteFile(composePath(), body, 0o644); err != nil {
		return err
	}
	return dockerCompose("up", "-d")
}

// Down stops Traefik (leaves the network in place).
func Down() error { return dockerCompose("down") }

// Running reports whether the lane-proxy container is up.
func Running() bool {
	out, _ := exec.Command("docker", "ps", "--filter", "name=^lane-proxy$", "--format", "{{.Names}}").Output()
	return bytes.Contains(out, []byte("lane-proxy"))
}

// Ensure starts the proxy only if it is not already running.
func Ensure() error {
	if Running() {
		return nil
	}
	return Up()
}

func dockerCompose(args ...string) error {
	full := append([]string{"compose", "-p", "lane-proxy", "-f", composePath()}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
