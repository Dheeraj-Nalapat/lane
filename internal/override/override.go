// Package override generates the non-invasive Compose overlay lane applies.
package override

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Route is a resolved web entrypoint (hostname already rendered).
type Route struct {
	Service  string
	Port     int
	Hostname string
}

// Spec is the input to Generate.
type Spec struct {
	Slug          string
	Project       string // manifest name, shared across a project's stacks
	ProjectPath   string
	Network       string // shared external network name, e.g. "lane"
	Services      []string
	Routes        []Route
	TiltPort      int
	TLS           bool
	BuiltServices []string
}

// resetNode emits a Compose `!reset` override. seq=true → `!reset []`,
// otherwise `!reset null`.
type resetNode struct{ seq bool }

func (r resetNode) MarshalYAML() (any, error) {
	if r.seq {
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!reset"}, nil
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!reset", Value: "null"}, nil
}

// Generate returns the override YAML bytes.
func Generate(s Spec) ([]byte, error) {
	routed := map[string]Route{}
	for _, r := range s.Routes {
		routed[r.Service] = r
	}
	built := map[string]bool{}
	for _, b := range s.BuiltServices {
		built[b] = true
	}

	idLabels := []string{
		"lane.managed=true",
		"lane.slug=" + s.Slug,
		"lane.project=" + s.Project,
		"lane.project.path=" + s.ProjectPath,
	}
	if s.TiltPort > 0 {
		idLabels = append(idLabels, fmt.Sprintf("lane.tilt.port=%d", s.TiltPort))
	}

	services := map[string]any{}
	svcNames := append([]string(nil), s.Services...)
	sort.Strings(svcNames)
	for _, name := range svcNames {
		svc := map[string]any{
			"container_name": resetNode{},          // !reset null
			"ports":          resetNode{seq: true}, // !reset []
		}
		if built[name] {
			svc["image"] = resetNode{} // !reset null → compose names the built image by project (slug)
		}
		labels := append([]string(nil), idLabels...)
		if r, ok := routed[name]; ok {
			svc["networks"] = []string{"default", s.Network}
			router := s.Slug + "-" + name
			labels = append(labels,
				"traefik.enable=true",
				"traefik.docker.network="+s.Network,
				fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", router, r.Hostname),
				"traefik.http.routers."+router+".entrypoints=web",
				fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%d", router, r.Port),
				"lane.url=http://"+r.Hostname,
			)
			if s.TLS {
				tlsRouter := router + "-tls"
				labels = append(labels,
					fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", tlsRouter, r.Hostname),
					"traefik.http.routers."+tlsRouter+".entrypoints=websecure",
					"traefik.http.routers."+tlsRouter+".tls=true",
					"traefik.http.routers."+tlsRouter+".service="+router,
				)
			}
		}
		svc["labels"] = labels
		services[name] = svc
	}

	doc := map[string]any{
		"services": services,
		"networks": map[string]any{
			s.Network: map[string]any{"external": true},
		},
	}
	return yaml.Marshal(doc)
}
