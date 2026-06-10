// Package routing merges explicit .lane.toml routes with auto-routes derived
// from the compose file (one host per HTTP service).
package routing

import (
	"sort"

	"github.com/dheeraj-nalapat/lane/internal/compose"
	"github.com/dheeraj-nalapat/lane/internal/override"
)

// Resolve returns the merged route set and the names of services that were
// eligible for auto-routing but had no single exposed port (skipped). Explicit
// routes always win; auto-routes use <slug>-<service>.localhost. When autoroute
// is false, only explicit routes are returned and skipped is empty.
func Resolve(slug string, services []compose.Service, explicit []override.Route, autoroute bool, exclude []string) (routes []override.Route, skipped []string) {
	routed := map[string]bool{}
	for _, r := range explicit {
		routed[r.Service] = true
		routes = append(routes, r)
	}
	if !autoroute {
		return routes, nil
	}
	ex := map[string]bool{}
	for _, e := range exclude {
		ex[e] = true
	}
	// Stable order so output (and any logging) is deterministic.
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	for _, s := range services {
		if routed[s.Name] || ex[s.Name] {
			continue
		}
		if s.Port == 0 {
			skipped = append(skipped, s.Name)
			continue
		}
		routes = append(routes, override.Route{
			Service:  s.Name,
			Port:     s.Port,
			Hostname: slug + "-" + s.Name + ".localhost",
		})
	}
	return routes, skipped
}
