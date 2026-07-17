package config

import (
	"fmt"
	"net/url"
	"strings"
)

func (cfg *AssignmentConfig) StartercodeURL() {
	url, err := cfg.gitURLToWebURL(cfg.Startercode.URL, cfg.Startercode.FromBranch)
	if err != nil {
		fmt.Printf("error: %s\n", err)
		return
	}
	fmt.Println(url)
}

func (cfg *AssignmentConfig) gitURLToWebURL(raw, branch string) (string, error) {
	base, err := gitURLToHTTPSBase(raw)
	if err != nil {
		return "", err
	}
	if branch == "" {
		return base, nil
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + "-/tree/" + url.PathEscape(branch), nil
}

// HTTPSCloneURL turns a starter-code or repository URL into the HTTPS clone URL
// glabs talks to. The SSH form `git@host:path.git` stays valid *notation* in a
// course file — it is what people paste out of GitLab — but glabs clones and
// pushes over HTTPS with a PAT, so every URL is normalized to https here.
func HTTPSCloneURL(raw string) (string, error) {
	base, err := gitURLToHTTPSBase(raw)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(base, ".git") {
		base += ".git"
	}
	return base, nil
}

// gitURLToHTTPSBase accepts either an https URL (returned as-is, minus any .git)
// or the SSH form git@host:path[.git], and returns https://host/path.
func gitURLToHTTPSBase(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty URL")
	}

	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		return strings.TrimSuffix(raw, ".git"), nil
	}

	if strings.HasPrefix(raw, "git@") {
		rest := strings.TrimPrefix(raw, "git@")
		host, path, found := strings.Cut(rest, ":")
		if !found {
			return "", fmt.Errorf("invalid SSH URL: %q", raw)
		}
		return "https://" + host + "/" + strings.TrimSuffix(path, ".git"), nil
	}

	return "", fmt.Errorf("unsupported URL format: %q", raw)
}

func (cfg *AssignmentConfig) Urls(assignment bool) {
	if assignment {
		fmt.Println(cfg.URL)
	} else if cfg.Per == PerStudent {
		for _, stud := range cfg.Students {
			fmt.Printf("%s/%s\n", cfg.URL, cfg.RepoNameForStudent(stud))
		}
	} else { // PerGroup
		for _, group := range cfg.Groups {
			fmt.Printf("%s/%s\n", cfg.URL, cfg.RepoNameForGroup(group))
		}
	}
}
