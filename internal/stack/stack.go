// Package stack holds the shared Stack model used by ls/view/down.
package stack

// Stack is one lane-managed project stack, aggregated from container labels.
type Stack struct {
	Slug        string   `json:"slug"`
	URL         string   `json:"url"`
	TiltPort    int      `json:"tiltPort"`
	ProjectPath string   `json:"path"`
	Containers  []string `json:"-"`
	Running     bool     `json:"running"`
}
