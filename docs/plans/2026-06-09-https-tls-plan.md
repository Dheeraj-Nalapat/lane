# HTTPS / TLS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add opt-in, trusted `https://<slug>.localhost` (mkcert) served by the shared proxy alongside plain HTTP, via a `lane tls` command — with no hard dependency on mkcert.

**Architecture:** A new `internal/tlsx` package owns cert paths, mkcert detection/generation, and the Traefik TLS config. The proxy template renders a `:443` entrypoint + cert mount only when the wildcard cert exists; the override generator adds a per-service `…-tls` router when TLS is on. `lane tls enable/disable/status` drives it; cert-presence is the single on/off signal.

**Tech Stack:** Go 1.22, Traefik file provider, mkcert (optional external tool). No new Go dependencies.

Spec: `docs/2026-06-09-https-tls-design.md`.

---

## File Structure

```
internal/paths/paths.go        + TraefikCerts(); include in Ensure()
internal/tlsx/tlsx.go           NEW — cert paths, Enabled, MkcertInstalled,
                                CAPresent, CertNames, mkcertArgs, Generate,
                                RenderTLSConfig, TLSConfigPath, Remove
internal/tlsx/tlsx_test.go      NEW — pure-func tests
internal/proxy/traefik-compose.yml.tmpl   conditional :443 + cert mount
internal/proxy/proxy.go         renderCompose(tls); Up derives from tlsx.Enabled
internal/proxy/proxy_test.go    renderCompose tls on/off
internal/override/override.go   Spec.TLS → per-service -tls router
internal/override/override_test.go
internal/tiltx/tiltx.go         RenderDynamicRoute(slug, port, tls)
internal/tiltx/tiltx_test.go
internal/runner/runner.go       RunSpec.TLS
internal/runner/tilt.go         pass s.TLS to RenderDynamicRoute (Up + DryRunLines)
cmd/up.go                       tls := tlsx.Enabled(); thread into override + RunSpec
cmd/tls.go                      NEW — lane tls enable|disable|status
README.md, CHANGELOG.md         document HTTPS
```

---

### Task 1: `paths.TraefikCerts()`

**Files:**
- Modify: `internal/paths/paths.go`
- Test: `internal/paths/paths_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/paths/paths_test.go` (inside `TestPaths`, after the `TraefikDynamic` check):
```go
	if !strings.HasSuffix(TraefikCerts(), "/traefik/certs") {
		t.Fatalf("TraefikCerts = %q", TraefikCerts())
	}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/paths/`
Expected: FAIL — `undefined: TraefikCerts`.

- [ ] **Step 3: Implement**

In `internal/paths/paths.go`, add after the `TraefikDynamic` func:
```go
func TraefikCerts() string { return filepath.Join(Traefik(), "certs") }
```
And add it to the `Ensure` loop:
```go
	for _, d := range []string{Overrides(), Run(), TraefikDynamic(), TraefikCerts()} {
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/paths/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/paths/
git commit -m "feat(paths): add TraefikCerts dir"
```

---

### Task 2: `internal/tlsx` package

**Files:**
- Create: `internal/tlsx/tlsx.go`, `internal/tlsx/tlsx_test.go`

- [ ] **Step 1: Write the failing tests (pure funcs)**

`internal/tlsx/tlsx_test.go`:
```go
package tlsx

import (
	"strings"
	"testing"
)

func TestCertNames(t *testing.T) {
	got := strings.Join(CertNames(), " ")
	want := "*.localhost *.*.localhost localhost"
	if got != want {
		t.Fatalf("CertNames = %q, want %q", got, want)
	}
}

func TestMkcertArgs(t *testing.T) {
	got := strings.Join(mkcertArgs("/c/cert.pem", "/c/key.pem"), " ")
	want := "-cert-file /c/cert.pem -key-file /c/key.pem *.localhost *.*.localhost localhost"
	if got != want {
		t.Fatalf("mkcertArgs = %q, want %q", got, want)
	}
}

func TestRenderTLSConfig(t *testing.T) {
	s := string(RenderTLSConfig())
	for _, want := range []string{"defaultCertificate", "/certs/cert.pem", "/certs/key.pem"} {
		if !strings.Contains(s, want) {
			t.Errorf("RenderTLSConfig missing %q:\n%s", want, s)
		}
	}
}

func TestEnabled_FollowsCert(t *testing.T) {
	t.Setenv("LANE_HOME", t.TempDir())
	if Enabled() {
		t.Fatal("Enabled() true with no cert")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tlsx/`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Implement**

`internal/tlsx/tlsx.go`:
```go
// Package tlsx manages optional mkcert-backed TLS for lane (https://*.localhost).
package tlsx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dheeraj-nalapat/lane/internal/paths"
)

// CertPath / KeyPath are the wildcard cert lane serves.
func CertPath() string { return filepath.Join(paths.TraefikCerts(), "cert.pem") }
func KeyPath() string  { return filepath.Join(paths.TraefikCerts(), "key.pem") }

// TLSConfigPath is the Traefik file-provider TLS config.
func TLSConfigPath() string { return filepath.Join(paths.TraefikDynamic(), "tls.yml") }

// Enabled reports whether TLS is on (the wildcard cert exists).
func Enabled() bool {
	_, err := os.Stat(CertPath())
	return err == nil
}

// MkcertInstalled reports whether the mkcert binary is on PATH.
func MkcertInstalled() bool {
	_, err := exec.LookPath("mkcert")
	return err == nil
}

// CAPresent reports whether a mkcert CA root exists (rootCA.pem under -CAROOT).
// Presence is a proxy for "the user has set up mkcert"; full trust-store
// installation still requires `mkcert -install`.
func CAPresent() bool {
	out, err := exec.Command("mkcert", "-CAROOT").Output()
	if err != nil {
		return false
	}
	caroot := strings.TrimSpace(string(out))
	if caroot == "" {
		return false
	}
	_, err = os.Stat(filepath.Join(caroot, "rootCA.pem"))
	return err == nil
}

// CertNames are the SANs lane requests for the wildcard cert.
func CertNames() []string {
	return []string{"*.localhost", "*.*.localhost", "localhost"}
}

func mkcertArgs(certPath, keyPath string) []string {
	return append([]string{"-cert-file", certPath, "-key-file", keyPath}, CertNames()...)
}

// Generate runs mkcert to write the wildcard cert/key (CA must already exist).
func Generate() error {
	if err := paths.Ensure(); err != nil {
		return err
	}
	cmd := exec.Command("mkcert", mkcertArgs(CertPath(), KeyPath())...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mkcert generate failed: %v\n%s", err, out)
	}
	return nil
}

// RenderTLSConfig is the Traefik file-provider config naming the default cert.
func RenderTLSConfig() []byte {
	return []byte(`tls:
  stores:
    default:
      defaultCertificate:
        certFile: /certs/cert.pem
        keyFile: /certs/key.pem
`)
}

// WriteTLSConfig writes the TLS dynamic config.
func WriteTLSConfig() error {
	if err := paths.Ensure(); err != nil {
		return err
	}
	return os.WriteFile(TLSConfigPath(), RenderTLSConfig(), 0o644)
}

// Remove deletes the cert, key, and TLS config (disables TLS).
func Remove() error {
	for _, p := range []string{CertPath(), KeyPath(), TLSConfigPath()} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/tlsx/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tlsx/
git commit -m "feat(tlsx): mkcert detection, wildcard cert generation, TLS config"
```

---

### Task 3: Proxy `:443` rendering

**Files:**
- Modify: `internal/proxy/traefik-compose.yml.tmpl`, `internal/proxy/proxy.go`
- Test: `internal/proxy/proxy_test.go`

- [ ] **Step 1: Update the template (conditional TLS blocks)**

Replace `internal/proxy/traefik-compose.yml.tmpl` with:
```yaml
services:
  proxy:
    image: traefik:v3.1
    container_name: lane-proxy
    command:
      - --providers.docker=true
      - --providers.docker.exposedByDefault=false
      - --providers.docker.network={{.Network}}
      - --providers.file.directory=/dynamic
      - --providers.file.watch=true
      - --entryPoints.web.address=:80
{{- if .TLS}}
      - --entryPoints.websecure.address=:443
{{- end}}
      - --api.dashboard=true
      - --api.insecure=true
    labels:
      - lane.managed=true
      - lane.proxy=true
    ports:
      - "80:80"
{{- if .TLS}}
      - "443:443"
{{- end}}
      - "127.0.0.1:8080:8080"
    extra_hosts:
      - "host.docker.internal:host-gateway"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - {{.DynamicDir}}:/dynamic:ro
{{- if .TLS}}
      - {{.CertsDir}}:/certs:ro
{{- end}}
    networks:
      - {{.Network}}
    restart: unless-stopped
networks:
  {{.Network}}:
    name: {{.Network}}
    external: true
```

- [ ] **Step 2: Write the failing test**

Replace the body of `internal/proxy/proxy_test.go`'s `TestRenderCompose` and add a TLS case:
```go
func TestRenderCompose_NoTLS(t *testing.T) {
	out, err := renderCompose("lane", "/d/dynamic", "/d/certs", false)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	s := string(out)
	for _, want := range []string{"image: traefik:v3.1", "--providers.docker.network=lane", "external: true"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, absent := range []string{":443", "websecure", "/certs"} {
		if strings.Contains(s, absent) {
			t.Errorf("TLS-off output unexpectedly contains %q:\n%s", absent, s)
		}
	}
}

func TestRenderCompose_TLS(t *testing.T) {
	out, err := renderCompose("lane", "/d/dynamic", "/d/certs", true)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	s := string(out)
	for _, want := range []string{"--entryPoints.websecure.address=:443", `"443:443"`, "/d/certs:/certs:ro"} {
		if !strings.Contains(s, want) {
			t.Errorf("TLS-on output missing %q:\n%s", want, s)
		}
	}
}
```
(Delete the old `TestRenderCompose` if it remains, to avoid a duplicate/!signature mismatch.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/proxy/`
Expected: FAIL — `renderCompose` arity mismatch / undefined.

- [ ] **Step 4: Update `renderCompose` + `Up`**

In `internal/proxy/proxy.go`, replace `renderCompose` and update `Up`:
```go
func renderCompose(network, dynamicDir, certsDir string, tls bool) ([]byte, error) {
	t, err := template.New("c").Parse(composeTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]any{
		"Network": network, "DynamicDir": dynamicDir, "CertsDir": certsDir, "TLS": tls,
	})
	return buf.Bytes(), err
}
```
And in `Up()`, replace the `renderCompose(...)` call:
```go
	body, err := renderCompose(Network, paths.TraefikDynamic(), paths.TraefikCerts(), tlsx.Enabled())
```
Add the import `"github.com/dheeraj-nalapat/lane/internal/tlsx"` to `proxy.go`.

- [ ] **Step 5: Run tests + build**

Run: `go test ./internal/proxy/ && go build ./...`
Expected: PASS, builds.

- [ ] **Step 6: Commit**

```bash
git add internal/proxy/
git commit -m "feat(proxy): render :443 entrypoint + cert mount when TLS enabled"
```

---

### Task 4: Override `…-tls` router

**Files:**
- Modify: `internal/override/override.go`
- Test: `internal/override/override_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/override/override_test.go`:
```go
func TestGenerate_TLSRouter(t *testing.T) {
	out, _ := Generate(Spec{
		Slug: "demo", ProjectPath: "/p", Network: "lane",
		Services: []string{"web"}, TiltPort: 0, TLS: true,
		Routes: []Route{{Service: "web", Port: 80, Hostname: "demo.localhost"}},
	})
	s := string(out)
	for _, want := range []string{
		"traefik.http.routers.demo-web-tls.rule=Host(`demo.localhost`)",
		"traefik.http.routers.demo-web-tls.entrypoints=websecure",
		"traefik.http.routers.demo-web-tls.tls=true",
		"traefik.http.routers.demo-web-tls.service=demo-web",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("TLS router missing %q:\n%s", want, s)
		}
	}
}

func TestGenerate_NoTLSRouterWhenOff(t *testing.T) {
	out, _ := Generate(Spec{
		Slug: "demo", ProjectPath: "/p", Network: "lane",
		Services: []string{"web"}, TLS: false,
		Routes: []Route{{Service: "web", Port: 80, Hostname: "demo.localhost"}},
	})
	if strings.Contains(string(out), "-tls") {
		t.Fatalf("TLS off must not emit a -tls router:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/override/ -run TLS`
Expected: FAIL — `Spec.TLS undefined`.

- [ ] **Step 3: Implement**

In `internal/override/override.go`, add `TLS bool` to `Spec` (after `TiltPort int`):
```go
	TiltPort    int
	TLS         bool
```
Then, inside the `if r, ok := routed[name]; ok {` block, after the existing `labels = append(labels, ...)` that ends with `"lane.url=http://"+r.Hostname,`, add:
```go
			if s.TLS {
				tlsRouter := router + "-tls"
				labels = append(labels,
					fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", tlsRouter, r.Hostname),
					"traefik.http.routers."+tlsRouter+".entrypoints=websecure",
					"traefik.http.routers."+tlsRouter+".tls=true",
					"traefik.http.routers."+tlsRouter+".service="+router,
				)
			}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/override/`
Expected: PASS (existing tests unaffected; TLS defaults false).

- [ ] **Step 5: Commit**

```bash
git add internal/override/
git commit -m "feat(override): add per-service websecure TLS router when enabled"
```

---

### Task 5: Tilt-UI route TLS + RunSpec.TLS

**Files:**
- Modify: `internal/tiltx/tiltx.go`, `internal/tiltx/tiltx_test.go`, `internal/runner/runner.go`, `internal/runner/tilt.go`

- [ ] **Step 1: Write the failing test**

Replace `TestRenderDynamic` in `internal/tiltx/tiltx_test.go` and add a TLS case:
```go
func TestRenderDynamic_NoTLS(t *testing.T) {
	out, err := RenderDynamicRoute("remind", 10377, false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "Host(`tilt-remind.localhost`)") {
		t.Errorf("missing web router:\n%s", s)
	}
	if strings.Contains(s, "websecure") || strings.Contains(s, "tls") {
		t.Errorf("no-TLS output must not contain websecure/tls:\n%s", s)
	}
}

func TestRenderDynamic_TLS(t *testing.T) {
	out, err := RenderDynamicRoute("remind", 10377, true)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	for _, want := range []string{"remind-tilt-tls", "websecure", "tls: true"} {
		if !strings.Contains(s, want) {
			t.Errorf("TLS output missing %q:\n%s", want, s)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tiltx/`
Expected: FAIL — `RenderDynamicRoute` arity mismatch.

- [ ] **Step 3: Implement the tiltx change**

Replace the template + function in `internal/tiltx/tiltx.go`:
```go
const dynamicTmpl = `http:
  routers:
    {{.Slug}}-tilt:
      rule: "Host(` + "`tilt-{{.Slug}}.localhost`" + `)"
      service: {{.Slug}}-tilt
      entryPoints: [web]
{{- if .TLS}}
    {{.Slug}}-tilt-tls:
      rule: "Host(` + "`tilt-{{.Slug}}.localhost`" + `)"
      service: {{.Slug}}-tilt
      entryPoints: [websecure]
      tls: true
{{- end}}
  services:
    {{.Slug}}-tilt:
      loadBalancer:
        servers:
          - url: "http://host.docker.internal:{{.Port}}"
`

// RenderDynamicRoute produces the Traefik file-provider config for the Tilt UI,
// adding a websecure/TLS router when tls is true.
func RenderDynamicRoute(slug string, port int, tls bool) ([]byte, error) {
	t, err := template.New("d").Parse(dynamicTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]any{"Slug": slug, "Port": port, "TLS": tls})
	return buf.Bytes(), err
}
```

- [ ] **Step 4: Add `TLS` to RunSpec + pass it through tiltRunner**

In `internal/runner/runner.go`, add to `RunSpec` (after `Env []string`):
```go
	TLS bool
```
In `internal/runner/tilt.go`, update both `RenderDynamicRoute` calls to pass `s.TLS`:
- in `DryRunLines`: `dyn, _ := tiltx.RenderDynamicRoute(s.Slug, s.TiltPort, s.TLS)`
- in `Up`: `dyn, err := tiltx.RenderDynamicRoute(s.Slug, s.TiltPort, s.TLS)`

- [ ] **Step 5: Run tests + build**

Run: `go test ./internal/tiltx/ ./internal/runner/ && go build ./...`
Expected: tiltx tests PASS; runner builds (cmd/up.go sets `.TLS` in Task 6 — `go build ./...` may fail until then, that's fine; re-run after Task 6).

- [ ] **Step 6: Commit**

```bash
git add internal/tiltx/ internal/runner/
git commit -m "feat(tiltx,runner): TLS router for the Tilt UI route"
```

---

### Task 6: Wire TLS into `cmd/up.go`

**Files:**
- Modify: `cmd/up.go`

- [ ] **Step 1: Set the TLS flag from cert presence**

In `cmd/up.go`, add the import `"github.com/dheeraj-nalapat/lane/internal/tlsx"`.

After the `routes` slice is built and before `override.Generate`, compute TLS once:
```go
	tlsOn := tlsx.Enabled()
```
Add `TLS: tlsOn` to the `override.Spec{...}` literal:
```go
	body, err := override.Generate(override.Spec{
		Slug: sl, ProjectPath: dir, Network: proxy.Network,
		Services: svcs, Routes: routes, TiltPort: tiltPort, TLS: tlsOn,
	})
```
Add `TLS: tlsOn` to the `runner.RunSpec{...}` literal (so the tilt route gets it):
```go
	spec := runner.RunSpec{
		Slug: sl, Dir: dir, ComposePath: composePath, OverridePath: overridePath,
		Routes: routes, Detach: flagDetach, Build: flagBuild,
		TiltPort: tiltPort, DynamicPath: dynamicPath, Env: env, TLS: tlsOn,
	}
```

- [ ] **Step 2: Build + full test**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: builds, all pass, vet clean.

- [ ] **Step 3: Commit**

```bash
git add cmd/up.go
git commit -m "feat(up): emit TLS routers when lane TLS is enabled"
```

---

### Task 7: `lane tls` command

**Files:**
- Create: `cmd/tls.go`

- [ ] **Step 1: Implement**

`cmd/tls.go`:
```go
package cmd

import (
	"errors"
	"fmt"

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
```

- [ ] **Step 2: Build + vet + test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: builds, vet clean, all tests pass.

- [ ] **Step 3: Manual (no Docker needed): mkcert-absent message**

```bash
go build -o ./bin/lane .
PATH=/usr/bin ./bin/lane tls enable    # if mkcert isn't in /usr/bin
```
Expected: prints the mkcert install instruction and exits non-zero (no cert written).

- [ ] **Step 4: Commit**

```bash
git add cmd/tls.go
git commit -m "feat(tls): lane tls enable|disable|status"
```

---

### Task 8: Docs

**Files:**
- Modify: `README.md`, `CHANGELOG.md`

- [ ] **Step 1: README — add an HTTPS section**

In `README.md`, after the "Commands" section, add:
```markdown
## HTTPS (optional)

lane serves plain HTTP by default. To get trusted `https://<slug>.localhost`
(for secure cookies / HTTPS-only APIs), install [mkcert](https://github.com/FiloSottile/mkcert),
set up its CA once, then enable TLS:

```bash
mkcert -install        # one-time; adds a local CA to your trust store
lane tls enable        # generates a wildcard cert, restarts the proxy on :443
lane up                # (re-up running stacks to add their HTTPS route)
```

Both `http://` and `https://` then serve every stack. `lane tls status` shows
the current state; `lane tls disable` returns to HTTP-only. mkcert is **not**
required for normal (HTTP) use — only to enable HTTPS.
```

- [ ] **Step 2: CHANGELOG — add the entry**

In `CHANGELOG.md`, under `## [Unreleased]` → `### Added`, append:
```markdown
- Optional HTTPS: `lane tls enable|disable|status` serves trusted
  `https://*.localhost` via mkcert (alongside HTTP; no redirect). mkcert is not
  a hard dependency.
```

- [ ] **Step 3: Commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: document optional HTTPS / lane tls"
```

---

### Task 9: Live smoke (manual, needs Docker + mkcert)

**No code. Acceptance test.**

- [ ] **Step 1: Install/build + enable**

```bash
go build -o ~/.local/bin/lane .
mkcert -install                 # if not already (one-time)
lane proxy up
lane tls status                 # mkcert/CA yes, cert no (yet)
lane tls enable
lane tls status                 # cert yes; proxy :443 yes
```

- [ ] **Step 2: Bring up a stack and verify both schemes**

Use any compose project with a `.lane.toml` (e.g. a `traefik/whoami` stack):
```bash
cd <project> && lane up      # compose runner, or `lane up -d` for tilt
SLUG=<slug>
curl -s  http://$SLUG.localhost/  -o /dev/null -w 'http  %{http_code}\n'
curl -s https://$SLUG.localhost/  -o /dev/null -w 'https %{http_code}\n'   # trusted cert → 200, no -k needed
```
Expected: both `200`; the https cert is trusted (mkcert CA).

- [ ] **Step 3: Disable + confirm HTTP still works**

```bash
lane tls disable
curl -s  http://$SLUG.localhost/ -o /dev/null -w 'http %{http_code}\n'   # 200
curl -s https://$SLUG.localhost/ -o /dev/null -w 'https %{http_code}\n' 2>&1 || echo "https down (expected)"
```
Expected: http `200`; https no longer served.

- [ ] **Step 4: Tear down test stack**

```bash
cd <project> && lane down
```

---

## Final verification

- [ ] `go test ./...` — all pass (paths, tlsx, proxy, override, tiltx, runner, plus existing).
- [ ] `go vet ./...` clean; `gofmt -l .` empty.
- [ ] `lane tls enable` with no mkcert → install message, non-zero exit, no cert written.
- [ ] With TLS off, `lane up --dry-run` override output contains no `-tls` router (backward compatible).
- [ ] Live smoke: https + http both 200 with TLS on; http-only after disable.
