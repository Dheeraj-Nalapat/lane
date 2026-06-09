// Package stack holds the shared Stack model used by ls/view/down.
package stack

// Stack is one lane-managed project stack, aggregated from container labels.
type Stack struct {
	Slug        string
	URL         string
	TiltPort    int
	ProjectPath string
	Containers  []string
	Running     bool
}
