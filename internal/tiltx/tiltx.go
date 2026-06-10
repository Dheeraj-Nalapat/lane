// Package tiltx builds Tilt invocations and the Traefik file-provider route
// that fronts the per-stack Tilt dashboard.
package tiltx

import (
	"bytes"
	"strconv"
	"text/template"
)

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

// RenderDynamicRoute produces the Traefik file-provider config routing
// tilt-<slug>.localhost to the host-side Tilt UI port, adding a websecure/TLS
// router when tls is true.
func RenderDynamicRoute(slug string, port int, tls bool) ([]byte, error) {
	t, err := template.New("d").Parse(dynamicTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]any{"Slug": slug, "Port": port, "TLS": tls})
	return buf.Bytes(), err
}

// UpArgs returns the args for `tilt up` in lane's docker mode on a given port.
// Tilt's own flags (--host, --port) and any resource names must come BEFORE the
// `--` separator; everything after `--` is passed to the Tiltfile's config.parse
// (here just --docker). --host 0.0.0.0 is required so the Tilt UI is reachable
// from the Traefik container via host.docker.internal (Tilt binds localhost by
// default).
func UpArgs(port int, resources []string) []string {
	args := []string{"up", "--host", "0.0.0.0", "--port", strconv.Itoa(port)}
	args = append(args, resources...)
	return append(args, "--", "--docker")
}
