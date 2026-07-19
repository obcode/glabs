package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/obcode/glabs/v3/web/db"
)

type sentMail struct{ to, subject string }

type fakeMailer struct {
	sent []sentMail
	fail bool
}

func (m *fakeMailer) Send(_ bool, to, subject string, _, _ []byte) error {
	if m.fail {
		return fmt.Errorf("smtp down")
	}
	m.sent = append(m.sent, sentMail{to, subject})
	return nil
}

func doneJob(id, owner string) *db.ScheduledJob {
	return &db.ScheduledJob{
		ID: id, Owner: owner, Op: "setaccess", Course: "mpd", Assignment: "blatt01",
		Status: db.JobDone, RunAt: time.Now(), GraceMin: 60,
	}
}

func TestNotifyFinishedJobs_sendsOnceAndMarks(t *testing.T) {
	fs := newFakeStore()
	fm := &fakeMailer{}
	a := &App{db: fs, mailer: fm}
	fs.jobs["j1"] = doneJob("j1", "prof@hm.edu")

	a.notifyFinishedJobs(context.Background())

	if len(fm.sent) != 1 {
		t.Fatalf("sent %d mails, want 1", len(fm.sent))
	}
	if fm.sent[0].to != "prof@hm.edu" || !strings.Contains(fm.sent[0].subject, "erfolgreich") {
		t.Errorf("mail = %+v, want to prof@hm.edu, subject 'erfolgreich'", fm.sent[0])
	}
	if !fs.jobs["j1"].Notified {
		t.Error("job should be marked notified after a successful send")
	}

	// A second sweep must not re-mail an already-notified job.
	a.notifyFinishedJobs(context.Background())
	if len(fm.sent) != 1 {
		t.Errorf("notified job was re-mailed (%d sends)", len(fm.sent))
	}
}

func TestNotifyFinishedJobs_failureLeavesUnnotifiedForRetry(t *testing.T) {
	fs := newFakeStore()
	fm := &fakeMailer{fail: true}
	a := &App{db: fs, mailer: fm}
	fs.jobs["j1"] = doneJob("j1", "prof@hm.edu")

	a.notifyFinishedJobs(context.Background())

	if fs.jobs["j1"].Notified {
		t.Error("a failed send must leave the job unnotified so the next tick retries")
	}
}

func TestNotifyFinishedJobs_noMailerIsNoop(t *testing.T) {
	fs := newFakeStore()
	a := &App{db: fs} // no mailer
	fs.jobs["j1"] = doneJob("j1", "prof@hm.edu")

	a.notifyFinishedJobs(context.Background()) // must not panic

	if fs.jobs["j1"].Notified {
		t.Error("with no mailer the job should not be marked notified")
	}
}

func TestScheduleOp_sendsConfirmation(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	withStoredToken(fs, owner)
	fm := &fakeMailer{}
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t), mailer: fm}
	ctx := ctxAs(owner)

	plan, err := a.PlanOp(ctx, "setaccess", "uc", "blatt1", nil, nil)
	if err != nil {
		t.Fatalf("PlanOp: %v", err)
	}
	if _, err := a.ScheduleOp(ctx, plan.Token, time.Now().Add(time.Hour), nil, ""); err != nil {
		t.Fatalf("ScheduleOp: %v", err)
	}
	if len(fm.sent) != 1 || !strings.Contains(fm.sent[0].subject, "geplant") {
		t.Errorf("scheduling confirmation not sent: %+v", fm.sent)
	}
}
