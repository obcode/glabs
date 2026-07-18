package app

import (
	"context"
	"fmt"

	"github.com/obcode/glabs/v3/gitlab"
	"github.com/obcode/glabs/v3/gitlab/report"
	"github.com/obcode/glabs/v3/reporter"
)

// gitlabClientFor builds a GitLab client from the caller's stored PAT. This is the
// single place in the web server where the plaintext token exists: it is
// decrypted here, handed straight to the client, and never logged or returned.
// The reporter receives the client's progress: a discard reporter for one-shot
// queries, or a streaming one for the report subscription.
func (a *App) gitlabClientFor(ctx context.Context, owner string, rep reporter.Reporter) (*gitlab.Client, error) {
	if a.sealer == nil {
		return nil, fmt.Errorf("secret storage is unavailable: set secrets.key in the server config")
	}
	sec, err := a.db.GetUserSecret(ctx, owner)
	if err != nil {
		return nil, err
	}
	if sec == nil || sec.GitLab == nil {
		return nil, fmt.Errorf("no GitLab token stored — add one under GitLab-Token first")
	}
	token, err := a.sealer.Open(*sec.GitLab)
	if err != nil {
		return nil, err
	}
	return gitlab.NewClient(
		gitlab.WithHost(a.gitlabHost),
		gitlab.WithToken(token),
		gitlab.WithReporter(rep),
	)
}

// AssignmentReport fetches a live report over the repositories of one assignment
// of one of the caller's courses, using the caller's stored GitLab token. It
// returns nil (no error) when there is no such assignment or it cannot be
// resolved (e.g. an abstract base); it errors when no token is stored or GitLab
// is unreachable.
func (a *App) AssignmentReport(ctx context.Context, course, name string) (*report.Reports, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := a.resolveAssignmentConfig(ctx, course, name)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}
	client, err := a.gitlabClientFor(ctx, o, reporter.NewDiscardReporter())
	if err != nil {
		return nil, err
	}
	return client.ReportData(cfg)
}
