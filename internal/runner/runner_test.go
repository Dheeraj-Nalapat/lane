package runner

import "testing"

func TestSelect(t *testing.T) {
	cases := []struct {
		name           string
		manifestRunner string
		tiltfile       bool
		want           string
	}{
		{"manifest forces compose", "compose", true, "compose"},
		{"manifest forces tilt", "tilt", false, "tilt"},
		{"auto: tiltfile present", "", true, "tilt"},
		{"auto: no tiltfile", "", false, "compose"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Select(c.manifestRunner, c.tiltfile); got != c.want {
				t.Errorf("Select(%q,%v) = %q, want %q", c.manifestRunner, c.tiltfile, got, c.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	if New("tilt").Name() != "tilt" {
		t.Fatal("New(tilt).Name != tilt")
	}
	if New("compose").Name() != "compose" {
		t.Fatal("New(compose).Name != compose")
	}
	if New("").Name() != "compose" {
		t.Fatal("New(\"\") should default to compose")
	}
}
