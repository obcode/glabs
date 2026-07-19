package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/gitlab"
	"github.com/obcode/glabs/v3/reporter"
)

// RunLine is one line of an operation's streamed output: a level (INFO, WARN,
// ERROR, PROGRESS, RESULT, DONE) plus text (may contain ANSI, though the reporter
// strips it).
type RunLine struct {
	Level string
	Text  string
}

// RunOp validates a plan token and runs the mutating GitLab operation it describes,
// streaming its output line by line. The pre-flight checks (token, config drift,
// seeder, confirm phrase, exclusive lock, GitLab token) fail synchronously with an
// error; once the operation starts, failures are streamed as an ERROR line.
//
// The operation runs on a context DETACHED from the subscription
// (context.WithoutCancel): closing the browser tab cancels the subscription (so the
// reporter stops streaming) but the operation keeps running to completion.
func (a *App) RunOp(ctx context.Context, token, confirmPhrase string) (<-chan RunLine, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}

	tok, err := a.openOpToken(token)
	if err != nil {
		return nil, err
	}
	if tok.Owner != o {
		return nil, fmt.Errorf("this plan was not created by you")
	}

	// Re-resolve and compare the config hash: reject a plan whose config changed
	// since it was made (the whole point of the token).
	cfg, err := a.resolveAssignmentConfig(ctx, tok.Course, tok.Assignment, tok.OnlyFor...)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("assignment %q of course %q no longer resolves", tok.Assignment, tok.Course)
	}
	hash, err := configHash(cfg)
	if err != nil {
		return nil, err
	}
	if hash != tok.ConfigHash {
		return nil, fmt.Errorf("the configuration changed since you planned this operation — please plan again")
	}

	if cfg.Seeder != nil {
		return nil, fmt.Errorf("assignment %q configures a seeder, which the web cannot run — use the CLI", tok.Assignment)
	}
	if tok.Op == "archive" || tok.Op == "delete" {
		want := tok.Course + "/" + tok.Assignment
		if confirmPhrase != want {
			return nil, fmt.Errorf("type %q to confirm this destructive operation", want)
		}
	}

	release, ok := a.ops.tryBegin(o, tok.Course, tok.Assignment)
	if !ok {
		return nil, fmt.Errorf("another operation on %s/%s is already running", tok.Course, tok.Assignment)
	}

	events := make(chan RunLine, 256)
	// Detached context: the op survives a client disconnect. The reporter still
	// uses the subscription ctx, so streaming stops once the client is gone.
	opCtx := context.WithoutCancel(ctx)
	rep := &opReporter{ctx: ctx, events: events}

	client, err := a.gitlabClientFor(opCtx, o, rep)
	if err != nil {
		release()
		return nil, err
	}

	go func() {
		defer close(events)
		defer release()
		rep.emit("INFO", fmt.Sprintf("running %s on %s/%s (%d repositories)", tok.Op, tok.Course, tok.Assignment, len(cfg.RepoTargets())))
		status, detail := "done", fmt.Sprintf("%d repositories", len(cfg.RepoTargets()))
		if err := executeOp(client, tok, cfg); err != nil {
			rep.emit("ERROR", err.Error())
			status, detail = "failed", err.Error()
		} else {
			rep.emit("RESULT", fmt.Sprintf("%s completed", tok.Op))
		}
		// Record the outcome in the activity log (best-effort — a logging failure
		// must not fail the op, so it is surfaced as a WARN and swallowed).
		if recErr := a.recordOp(opCtx, o, tok, status, detail); recErr != nil {
			rep.emit("WARN", "could not record activity: "+recErr.Error())
		}
		rep.emit("DONE", "done")
	}()

	return events, nil
}

// executeOp applies the op's parameters to the resolved config and dispatches to
// the corresponding GitLab client method. None of these touch git.
func executeOp(client *gitlab.Client, tok *opToken, cfg *config.AssignmentConfig) error {
	switch tok.Op {
	case "setaccess":
		if lvl := tok.Params["accessLevel"]; lvl != "" {
			cfg.SetAccessLevel(lvl)
		}
		return client.Setaccess(cfg)
	case "protect":
		if br := tok.Params["branch"]; br != "" {
			cfg.SetProtectToBranch(br)
		}
		return client.ProtectToBranch(cfg)
	case "archive":
		return client.Archive(cfg, tok.Params["unarchive"] == "true")
	case "delete":
		return client.Delete(cfg)
	}
	return fmt.Errorf("unknown operation %q", tok.Op)
}

// opReporter is a reporter.Reporter that classifies the GitLab client's progress
// into levelled RunLines and forwards them (ANSI stripped) on the subscription
// context, so a disconnected client never blocks the operation.
type opReporter struct {
	ctx    context.Context
	events chan<- RunLine
}

func (r *opReporter) emit(level, msg string) {
	msg = strings.TrimSpace(ansiRE.ReplaceAllString(msg, ""))
	if msg == "" {
		return
	}
	select {
	case r.events <- RunLine{Level: level, Text: msg}:
	case <-r.ctx.Done():
	}
}

func (r *opReporter) Printf(format string, a ...any) { r.emit("INFO", fmt.Sprintf(format, a...)) }
func (r *opReporter) Println(a ...any)               { r.emit("INFO", fmt.Sprintln(a...)) }
func (r *opReporter) Task(description string) reporter.Task {
	r.emit("PROGRESS", description)
	return &opTask{r}
}

type opTask struct{ r *opReporter }

func (t *opTask) Update(message string) { t.r.emit("PROGRESS", message) }
func (t *opTask) Done(message string)   { t.r.emit("INFO", message) }
func (t *opTask) Fail(message string)   { t.r.emit("ERROR", message) }
