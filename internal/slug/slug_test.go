package slug

import "testing"

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"ReMind":        "remind",
		"feature/Foo_X": "feature-foo-x",
		"--weird--":     "weird",
		"a..b__c":       "a-b-c",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitize_CapsLength(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "a"
	}
	if got := Sanitize(long); len(got) != 40 {
		t.Fatalf("len = %d, want 40", len(got))
	}
}

func TestResolve_Ladder(t *testing.T) {
	tests := []struct {
		name string
		in   Inputs
		want string
	}{
		{"flag wins", Inputs{Flag: "Foo", Env: "bar", ManifestName: "baz", DirBase: "qux"}, "foo"},
		{"env next", Inputs{Env: "Bar", ManifestName: "baz"}, "bar"},
		{"manifest main", Inputs{ManifestName: "remind"}, "remind"},
		{"manifest worktree", Inputs{ManifestName: "remind", Worktree: "featx"}, "remind-featx"},
		{"dir fallback", Inputs{DirBase: "myproj", Worktree: "wt"}, "myproj-wt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Resolve(tc.in); got != tc.want {
				t.Errorf("Resolve = %q, want %q", got, tc.want)
			}
		})
	}
}
