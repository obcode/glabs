package config

import (
	"fmt"
	"net/url"
	"strconv"
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
		return
	}
	for _, r := range cfg.RepoURLs() {
		fmt.Println(r.URL)
	}
}

// RepoTarget is one repository the assignment resolves to: who it belongs to
// (For — a student's email/username/id, or the group name), the repository's
// project name (Repo), and the full web URL.
type RepoTarget struct {
	For  string
	Repo string
	URL  string
}

// RepoTargets returns the per-student or per-group repositories for the assignment
// (depending on Per). It is the data behind both the printing Urls method and the
// web layer's URL list and operation targets.
func (cfg *AssignmentConfig) RepoTargets() []RepoTarget {
	var out []RepoTarget
	if cfg.Per == PerStudent {
		for _, stud := range cfg.Students {
			repo := cfg.RepoNameForStudent(stud)
			out = append(out, RepoTarget{For: studentLabel(stud), Repo: repo, URL: cfg.URL + "/" + repo})
		}
	} else { // PerGroup
		for _, group := range cfg.Groups {
			repo := cfg.RepoNameForGroup(group)
			out = append(out, RepoTarget{For: group.Name, Repo: repo, URL: cfg.URL + "/" + repo})
		}
	}
	return out
}

// RepoURL is one repository URL together with who it belongs to.
type RepoURL struct {
	For string
	URL string
}

// RepoURLs returns the per-student or per-group repository URLs for the
// assignment (a projection of RepoTargets). The assignment-level group URL is
// cfg.URL.
func (cfg *AssignmentConfig) RepoURLs() []RepoURL {
	targets := cfg.RepoTargets()
	out := make([]RepoURL, 0, len(targets))
	for _, t := range targets {
		out = append(out, RepoURL{For: t.For, URL: t.URL})
	}
	return out
}

// studentLabel is a human-readable identifier for a student: the email, else the
// username, else the raw roster entry, else the numeric id.
func studentLabel(s *Student) string {
	if s.Email != nil && *s.Email != "" {
		return *s.Email
	}
	if s.Username != nil && *s.Username != "" {
		return *s.Username
	}
	if s.Raw != "" {
		return s.Raw
	}
	if s.Id != nil {
		return strconv.Itoa(*s.Id)
	}
	return ""
}
