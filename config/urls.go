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
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return "", fmt.Errorf("leere URL")
	}

	// Bereits Web-URL -> direkt zurückgeben
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		base := raw
		if branch == "" {
			return base, nil
		}
		encBranch := url.PathEscape(branch)
		if !strings.HasSuffix(base, "/") {
			base += "/"
		}
		return base + "-/tree/" + encBranch, nil
	}

	// Git SSH Form: git@host:path.git
	if strings.HasPrefix(raw, "git@") {
		rest := strings.TrimPrefix(raw, "git@")
		i := strings.Index(rest, ":")
		if i < 0 {
			return "", fmt.Errorf("ungültige SSH-URL: %q", raw)
		}

		host := rest[:i]
		path := rest[i+1:]
		path = strings.TrimSuffix(path, ".git")

		base := "https://" + host + "/" + path
		if branch == "" {
			return base, nil
		}
		encBranch := url.PathEscape(branch)
		return base + "/-/tree/" + encBranch, nil
	}

	return "", fmt.Errorf("nicht unterstütztes URL-Format: %q", raw)
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
