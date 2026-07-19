package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/web/secrets"
)

// validOps are the mutating GitLab operations the web can plan and run. All but
// generate are pure GitLab API calls; generate also pushes starter code over git
// (in memory, so no server disk is involved).
var validOps = map[string]bool{"setaccess": true, "protect": true, "archive": true, "delete": true, "generate": true}

// PlannedTarget is one repository an operation would touch.
type PlannedTarget struct {
	For  string
	Repo string
	URL  string
}

// OpPlan is the preview of a mutating operation before it runs: the resolved
// config, the repositories it would touch, warnings, and an opaque confirm token
// carrying a hash of the resolved config — so a later runOp can reject a plan whose
// config changed underneath it (strictly stronger than the CLI's Scanln gate).
type OpPlan struct {
	Op            string
	Course        string
	Assignment    string
	Resolved      string
	Targets       []PlannedTarget
	Warnings      []string
	Destructive   bool
	ConfirmPhrase string
	Token         string
	ExpiresAt     time.Time
}

// opToken is the sealed payload behind an OpPlan.Token. It is AES-256-GCM sealed
// with the server key, so it is tamper-proof and opaque to the client. runOp opens
// it, checks the expiry, re-resolves the assignment and compares ConfigHash.
type opToken struct {
	Owner      string            `json:"owner"`
	Op         string            `json:"op"`
	Course     string            `json:"course"`
	Assignment string            `json:"assignment"`
	Params     map[string]string `json:"params,omitempty"`
	OnlyFor    []string          `json:"onlyFor,omitempty"`
	ConfigHash string            `json:"configHash"`
	Expires    int64             `json:"exp"`
}

const opTokenTTL = 5 * time.Minute

// PlanOp resolves an assignment and returns the plan for a mutating operation —
// which repositories it would touch and any warnings — plus a confirm token. It
// does NOT touch GitLab; it is a pure, token-free preview. A missing/unresolvable
// assignment or an unknown op is an error.
func (a *App) PlanOp(ctx context.Context, op, course, assignment string, params map[string]string, onlyFor []string) (*OpPlan, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	if !validOps[op] {
		return nil, fmt.Errorf("unknown operation %q", op)
	}
	if a.sealer == nil {
		return nil, fmt.Errorf("secret storage is unavailable: set secrets.key in the server config")
	}

	cfg, err := a.resolveAssignmentConfig(ctx, course, assignment, onlyFor...)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("assignment %q of course %q does not exist or cannot be resolved", assignment, course)
	}

	hash, err := configHash(cfg)
	if err != nil {
		return nil, err
	}

	targets := make([]PlannedTarget, 0)
	for _, t := range cfg.RepoTargets() {
		targets = append(targets, PlannedTarget{For: t.For, Repo: t.Repo, URL: t.URL})
	}

	var warnings []string
	if cfg.Seeder != nil {
		warnings = append(warnings, "This assignment configures a seeder, which the web cannot run — use the CLI for seeding.")
	}
	if len(targets) == 0 {
		warnings = append(warnings, "No students or groups — this operation would touch no repositories.")
	}

	now := time.Now()
	token, err := a.sealOpToken(opToken{
		Owner:      o,
		Op:         op,
		Course:     course,
		Assignment: assignment,
		Params:     params,
		OnlyFor:    onlyFor,
		ConfigHash: hash,
		Expires:    now.Add(opTokenTTL).Unix(),
	})
	if err != nil {
		return nil, err
	}

	plan := &OpPlan{
		Op:          op,
		Course:      course,
		Assignment:  assignment,
		Resolved:    cfg.Show(),
		Targets:     targets,
		Warnings:    warnings,
		Destructive: op == "archive" || op == "delete",
		Token:       token,
		ExpiresAt:   now.Add(opTokenTTL),
	}
	// For destructive ops the user must additionally type this phrase (GitHub
	// pattern) before runOp will proceed.
	if plan.Destructive {
		plan.ConfirmPhrase = course + "/" + assignment
	}
	return plan, nil
}

// configHash is a stable hash of the resolved config. JSON of the struct is
// deterministic (map keys sorted, slice order preserved), unlike Show() whose
// deferred-branch rendering varies between runs.
func configHash(cfg *config.AssignmentConfig) (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("cannot hash the resolved config: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// sealOpToken seals the payload into an opaque, tamper-proof token string.
func (a *App) sealOpToken(tok opToken) (string, error) {
	payload, err := json.Marshal(tok)
	if err != nil {
		return "", err
	}
	sealed, err := a.sealer.Seal(string(payload))
	if err != nil {
		return "", err
	}
	wire, err := json.Marshal(sealed)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(wire), nil
}

// openOpToken opens a token back to its payload and checks the expiry. It does NOT
// compare the config hash — that is runOp's job, after re-resolving.
func (a *App) openOpToken(token string) (*opToken, error) {
	if a.sealer == nil {
		return nil, fmt.Errorf("secret storage is unavailable: set secrets.key in the server config")
	}
	wire, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}
	var sealed secrets.SealedValue
	if err := json.Unmarshal(wire, &sealed); err != nil {
		return nil, fmt.Errorf("invalid token")
	}
	payload, err := a.sealer.Open(sealed)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}
	var tok opToken
	if err := json.Unmarshal([]byte(payload), &tok); err != nil {
		return nil, fmt.Errorf("invalid token")
	}
	if time.Now().Unix() > tok.Expires {
		return nil, fmt.Errorf("this plan has expired — create a new one")
	}
	return &tok, nil
}
