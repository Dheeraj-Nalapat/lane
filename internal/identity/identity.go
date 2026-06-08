// Package identity renders hostnames from route templates and a slug.
package identity

import "strings"

// RenderHost expands a host template like "{slug}" or "api.{slug}" into a full
// "<...>.localhost" hostname.
func RenderHost(tmpl, slug string) string {
	return strings.ReplaceAll(tmpl, "{slug}", slug) + ".localhost"
}
