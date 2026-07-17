package config

import "testing"

func TestHTTPSCloneURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"ssh notation", "git@gitlab.lrz.de:mpd/startercode/blatt-01.git", "https://gitlab.lrz.de/mpd/startercode/blatt-01.git"},
		{"ssh without .git", "git@gitlab.lrz.de:mpd/startercode/blatt-01", "https://gitlab.lrz.de/mpd/startercode/blatt-01.git"},
		{"https already", "https://gitlab.lrz.de/mpd/startercode/blatt-01.git", "https://gitlab.lrz.de/mpd/startercode/blatt-01.git"},
		{"https without .git", "https://gitlab.lrz.de/mpd/startercode/blatt-01", "https://gitlab.lrz.de/mpd/startercode/blatt-01.git"},
		{"whitespace", "  git@gitlab.lrz.de:mpd/x.git  ", "https://gitlab.lrz.de/mpd/x.git"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HTTPSCloneURL(tt.in)
			if err != nil {
				t.Fatalf("HTTPSCloneURL(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("HTTPSCloneURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHTTPSCloneURLErrors(t *testing.T) {
	for _, in := range []string{"", "  ", "ssh://gitlab.lrz.de/x", "git@nocolon"} {
		if _, err := HTTPSCloneURL(in); err == nil {
			t.Errorf("HTTPSCloneURL(%q) = nil error, want an error", in)
		}
	}
}
