package config

import "testing"

func TestAccessLevelString(t *testing.T) {
	tests := []struct {
		level AccessLevel
		want  string
	}{
		{Guest, "guest"},
		{Reporter, "reporter"},
		{Developer, "developer"},
		{Maintainer, "maintainer"},
		{AccessLevel(99), "maintainer"}, // default: anything != 10/20/30 returns maintainer
	}

	for _, tc := range tests {
		got := tc.level.String()
		if got != tc.want {
			t.Errorf("AccessLevel(%d).String() = %q, want %q", tc.level, got, tc.want)
		}
	}
}
