package mail

import (
	"strings"
	"testing"
)

// allTemplates is the catalogue of job-notification templates; every one must
// render cleanly against the sample data.
var allTemplates = []string{TmplScheduled, TmplDone, TmplFailed, TmplExpired}

func TestAllTemplatesRender(t *testing.T) {
	data := SampleJob()
	data.Err = "boom: 404 Not Found" // exercise the failed/expired branches too
	for _, name := range allTemplates {
		text, html, err := Render(name, data)
		if err != nil {
			t.Fatalf("%s: render error: %v", name, err)
		}
		if len(text) == 0 || len(html) == 0 {
			t.Errorf("%s: empty text (%d) or html (%d)", name, len(text), len(html))
		}
		// missingkey=error should prevent this, but guard the rendered output too.
		if strings.Contains(string(text), "<no value>") || strings.Contains(string(html), "<no value>") {
			t.Errorf("%s: output contains \"<no value>\" (an unfilled template field)", name)
		}
		// The HTML part must be wrapped in the base layout.
		if !strings.Contains(string(html), "<!DOCTYPE html>") || !strings.Contains(string(html), "glabs") {
			t.Errorf("%s: html not wrapped in the base layout:\n%s", name, html)
		}
		// The plain-text part carries the shared footer.
		if !strings.Contains(string(text), "automatisch von glabs erzeugt") {
			t.Errorf("%s: text missing the footer", name)
		}
	}
}

func TestRender_missingTemplateIsError(t *testing.T) {
	if _, _, err := Render("does-not-exist.md.tmpl", SampleJob()); err == nil {
		t.Error("rendering an unknown template should error")
	}
}

func TestRender_carriesData(t *testing.T) {
	text, _, err := Render(TmplFailed, SampleJob())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{"setaccess", "mpd/blatt01"} {
		if !strings.Contains(string(text), want) {
			t.Errorf("failed mail missing %q:\n%s", want, text)
		}
	}
}

func TestBuildMsg(t *testing.T) {
	s := NewSender(Config{From: "glabs@cs.hm.edu", Host: "smtp.example"})
	msg, err := s.buildMsg("prof@hm.edu", "Betreff", []byte("text"), []byte("<p>html</p>"))
	if err != nil {
		t.Fatalf("buildMsg: %v", err)
	}
	if from := msg.GetFromString(); len(from) != 1 || !strings.Contains(from[0], "glabs@cs.hm.edu") {
		t.Errorf("From = %v, want glabs@cs.hm.edu", from)
	}
	if to := msg.GetToString(); len(to) != 1 || !strings.Contains(to[0], "prof@hm.edu") {
		t.Errorf("To = %v, want prof@hm.edu", to)
	}
}

func TestBuildMsg_requiresFrom(t *testing.T) {
	s := NewSender(Config{Host: "smtp.example"}) // no From
	if _, err := s.buildMsg("prof@hm.edu", "s", []byte("t"), nil); err == nil {
		t.Error("buildMsg without a From address should fail")
	}
}

// A dry-run without a test recipient is an error, not a silent send to the real
// address — and it must fail before any network call.
func TestSend_dryRunRequiresTestRecipient(t *testing.T) {
	s := NewSender(Config{From: "glabs@cs.hm.edu", Host: "smtp.invalid"})
	if err := s.Send(true, "prof@hm.edu", "s", []byte("t"), []byte("h")); err == nil {
		t.Error("dry-run without a test recipient should fail")
	}
}
