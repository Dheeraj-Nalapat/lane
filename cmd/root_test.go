package cmd

import (
	"bytes"
	"testing"
)

func TestRootCommand_HasName(t *testing.T) {
	if root.Use != "lane" {
		t.Fatalf("root.Use = %q, want %q", root.Use, "lane")
	}
}

func TestRootCommand_HelpRuns(t *testing.T) {
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("--help produced no output")
	}
}
