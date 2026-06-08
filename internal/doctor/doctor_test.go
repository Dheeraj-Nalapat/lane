package doctor

import "testing"

func TestComposeOK(t *testing.T) {
	cases := map[string]bool{
		"Docker Compose version v2.40.1": true,
		"Docker Compose version v2.20.0": true,
		"Docker Compose version v2.19.9": false,
		"Docker Compose version v1.29.2": false,
		"garbage":                        false,
	}
	for in, want := range cases {
		if got := composeOK(in); got != want {
			t.Errorf("composeOK(%q) = %v, want %v", in, got, want)
		}
	}
}
