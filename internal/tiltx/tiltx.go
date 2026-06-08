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
  services:
    {{.Slug}}-tilt:
      loadBalancer:
        servers:
          - url: "http://host.docker.internal:{{.Port}}"
`

// RenderDynamicRoute produces the Traefik file-provider config routing
// tilt-<slug>.localhost to the host-side Tilt UI port.
func RenderDynamicRoute(slug string, port int) ([]byte, error) {
	t, err := template.New("d").Parse(dynamicTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]any{"Slug": slug, "Port": port})
	return buf.Bytes(), err
}

// UpArgs returns the args for `tilt up` in berth's docker mode on a given port.
func UpArgs(port int) []string {
	return []string{"up", "--", "--docker", "--port", strconv.Itoa(port)}
}
