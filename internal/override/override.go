// Package override generates the non-invasive Compose overlay berth applies.
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
	Slug        string
	ProjectPath string
	Network     string // shared external network name, e.g. "berth"
	Services    []string
	Routes      []Route
	TiltPort    int
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

	idLabels := []string{
		"berth.managed=true",
		"berth.slug=" + s.Slug,
		"berth.project.path=" + s.ProjectPath,
		fmt.Sprintf("berth.tilt.port=%d", s.TiltPort),
	}

	services := map[string]any{}
	svcNames := append([]string(nil), s.Services...)
	sort.Strings(svcNames)
	for _, name := range svcNames {
		svc := map[string]any{
			"container_name": resetNode{},          // !reset null
			"ports":          resetNode{seq: true}, // !reset []
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
				"berth.url=http://"+r.Hostname,
			)
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
