package cmd

import (
	"strings"
	"testing"
)

func TestActionArgs(t *testing.T) {
	got := strings.Join(actionArgs("restart", "remind"), " ")
	if got != "restart --slug remind" {
		t.Fatalf("actionArgs = %q", got)
	}
}
