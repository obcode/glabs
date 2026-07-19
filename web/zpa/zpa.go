// Package zpa is a minimal read-only client for HM's ZPA (Prüfungsamt) REST API,
// used to enrich a course roster with student details (name, group, gender). It
// is deliberately small — glabs only needs to look a student up — and keeps the
// HTTP details confined here, mirroring plexams.go's zpa package.
//
// glabs identifies students by email, but ZPA's get_student_info search is
// primarily used with a Matrikelnummer in plexams. StudentByEmail therefore
// looks up defensively (by full email, then the email's local part) and returns
// nil when ZPA has no unambiguous match — so the students page degrades to just
// the email rather than showing wrong data.
package zpa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config is the ZPA connection, read from the server config in bootstrap.
type Config struct {
	BaseURL string
	Token   string
}

// Client talks to the ZPA REST API with a fixed token.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New builds a ZPA client. baseURL and token are required by the caller; an empty
// config means ZPA is disabled (bootstrap passes nil then).
func New(cfg Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		token:   cfg.Token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Student is one ZPA student record.
type Student struct {
	Mtknr     string `json:"mtknr"`
	Greeting  string `json:"greeting"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Gender    string `json:"gender"`
	Group     string `json:"group"`
}

// StudentByEmail looks a student up defensively: first by the full email, then by
// the email's local part. It returns (nil, nil) when ZPA has no unambiguous match,
// so callers can show just the email rather than guessing.
func (c *Client) StudentByEmail(ctx context.Context, email string) (*Student, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}

	students, err := c.search(ctx, email)
	if err != nil {
		return nil, err
	}
	if s := pickMatch(students, email); s != nil {
		return s, nil
	}

	// Fallback: the local part (HM emails are name-based, e.g. a.ciftci@hm.edu).
	if local, _, ok := strings.Cut(email, "@"); ok && local != "" {
		students, err = c.search(ctx, local)
		if err != nil {
			return nil, err
		}
		if s := pickMatch(students, email); s != nil {
			return s, nil
		}
	}
	return nil, nil
}

// pickMatch prefers an exact (case-insensitive) email match; failing that, a sole
// result; otherwise nil (an ambiguous search must not enrich with the wrong
// person).
func pickMatch(students []*Student, email string) *Student {
	for _, s := range students {
		if s != nil && strings.EqualFold(strings.TrimSpace(s.Email), email) {
			return s
		}
	}
	if len(students) == 1 {
		return students[0]
	}
	return nil
}

// search calls get_student_info?ask=<ask>.
func (c *Client) search(ctx context.Context, ask string) ([]*Student, error) {
	var students []*Student
	err := c.get(ctx, "get_student_info?ask="+url.QueryEscape(ask), &students)
	return students, err
}

func (c *Client) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+path, nil)
	if err != nil {
		return fmt.Errorf("cannot build ZPA request for %s: %w", path, err)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Authorization", "Token "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach ZPA for %s: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ZPA returned %s for %s: %s", resp.Status, path, bodySnippet(body))
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("ZPA returned an unexpected (non-JSON) response for %s: %s", path, bodySnippet(body))
	}
	return nil
}

func bodySnippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	const max = 300
	if len(s) > max {
		s = s[:max] + " …(truncated)"
	}
	return s
}
