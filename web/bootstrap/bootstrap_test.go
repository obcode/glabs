package bootstrap

import "testing"

func TestCheckGitlabHost(t *testing.T) {
	for host, wantErr := range map[string]bool{
		"":                           false,
		"https://gitlab.example.org": false,
		"http://localhost:8929":      false,
		"gitlab.example.org":         true,
		"//gitlab.example.org":       true,
		"ftp://gitlab.example.org":   true,
		"https://":                   true,
	} {
		if err := checkGitlabHost(host); (err != nil) != wantErr {
			t.Errorf("checkGitlabHost(%q) = %v, want error: %v", host, err, wantErr)
		}
	}
}
