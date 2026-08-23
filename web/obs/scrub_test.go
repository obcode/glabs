package obs

import (
	"testing"

	sentry "github.com/getsentry/sentry-go"
)

// scrub is the zero-value scrubber, i.e. the production configuration.
var scrub = scrubber{}

func event() *sentry.Event {
	e := sentry.NewEvent()
	e.Logger = zerologLogger
	return e
}

// The test that carries the whole design: a tag nobody anticipated must not get
// out. It uses a key that exists nowhere in this code base on purpose — if the
// filter were a deny list, this would sail straight through.
func TestATagNobodyAnticipatedIsDropped(t *testing.T) {
	e := event()
	e.Tags["caller"] = "gitlab/archive.go:47"
	e.Tags["course"] = "mpd"
	e.Tags["voellig_neues_feld"] = "irgendwas Personenbezogenes"

	got := scrub.scrub(e, nil)

	if _, ok := got.Tags["voellig_neues_feld"]; ok {
		t.Error("an unknown tag survived the scrubber")
	}
	if got.Tags["course"] != "mpd" || got.Tags["caller"] != "gitlab/archive.go:47" {
		t.Errorf("an allowed tag was dropped: %v", got.Tags)
	}
}

// The names that must never leave, spelled out, so that adding one to
// allowedTags breaks a test rather than a student's privacy.
//
// project and projectPath are the ones specific to this repository: a generated
// project is named <assignment>-<student>, so they identify a person however
// structural they sound. The shared gitlab package logs both at Error level.
func TestTheIdentifyingTagsAreNotAllowed(t *testing.T) {
	for _, key := range []string{
		"email", "student", "user", "name",
		"project", "projectPath", "group", "groupPath",
		"url", "searchPattern", "token",
	} {
		if allowedTags[key] {
			t.Errorf("%q is on the allow list and must not be", key)
		}
	}
}

func TestMailAddressesAreRedactedInFreeText(t *testing.T) {
	e := event()
	e.Message = "cannot archive project for alice@example.org"
	e.Exception = []sentry.Exception{{
		Type:  "lookup failed",
		Value: "no member Vorname.Nachname@hm.edu",
	}}

	got := scrub.scrub(e, nil)

	if got.Message != "cannot archive project for [email]" {
		t.Errorf("message = %q", got.Message)
	}
	if got.Exception[0].Value != "no member [email]" {
		t.Errorf("exception value = %q", got.Exception[0].Value)
	}
}

// The glabs-specific one, and the reason this scrubber is not a copy.
//
// glabs names a generated project <assignment>-<student>, replacing '@' with
// '_at_' for filesystem compatibility (docs/configuration.md). The result
// contains no '@' at all, so the ordinary mail pattern does not fire on it —
// while it identifies the student exactly as well as the address it was made
// from. GitLab quotes these paths back in its error responses.
func TestTheAtSchemeInProjectPathsIsRedacted(t *testing.T) {
	// The whole token goes, assignment prefix included — see reAtScheme on why a
	// pattern that keeps the prefix would leak half of a hyphenated name.
	cases := map[string]string{
		"404 {message: 404 Project Not Found} for mpd-blatt01-alice_at_example.org": "404 {message: 404 Project Not Found} for [email]",
		"cannot clone se-blatt03-max.mustermann_at_hm.edu":                          "cannot clone [email]",
		"cannot clone mpd-blatt01-anna-lena_at_hm.edu":                              "cannot clone [email]",
	}
	for in, want := range cases {
		if got := redact(in); got != want {
			t.Errorf("redact(%q)\n got  %q\n want %q", in, got, want)
		}
	}
}

// The residual risk, written down as a test so it is a decision rather than an
// oversight: a student identified by a bare GitLab username is NOT redacted.
// Catching it would mean eating every <word>-<word> token, including the
// assignment and course names that make a report readable. The allow list is
// what carries the weight there.
func TestABareUsernameIsKnowinglyNotRedacted(t *testing.T) {
	const in = "cannot archive mpd-blatt01-alice"
	if got := redact(in); got != in {
		t.Errorf("redact(%q) = %q — if this now redacts, the documented trade-off changed", in, got)
	}
}

// A digit run must survive: unlike plexams, glabs has no Matrikelnummern, and
// eating numbers would only make GitLab ids and durations unreadable.
func TestDigitsSurvive(t *testing.T) {
	e := event()
	e.Message = "gitlab project 1787419 failed after 1234567 ms"

	if got := scrub.scrub(e, nil); got.Message != e.Message {
		t.Errorf("digits were redacted: %q", got.Message)
	}
}

func TestTheRequestIsRebuiltAndTheQueryThrownAway(t *testing.T) {
	e := event()
	e.Request = &sentry.Request{
		Method:      "POST",
		URL:         "https://glabs.cs.hm.edu/query?student=Nachname&token=geheim",
		QueryString: "student=Nachname&token=geheim",
		Cookies:     "session=abc",
		Data:        `{"query":"mutation { generate }"}`,
		Headers: map[string]string{
			"X-Remote-User": "oliver.braun@hm.edu",
			"Cookie":        "session=abc",
			"User-Agent":    "Mozilla/5.0",
		},
		Env: map[string]string{"REMOTE_ADDR": "10.0.0.1"},
	}

	r := scrub.scrub(e, nil).Request

	if r.QueryString != "" || r.Cookies != "" || r.Data != "" || len(r.Env) != 0 {
		t.Errorf("the rebuilt request kept something: %+v", r)
	}
	if r.URL != "https://glabs.cs.hm.edu/query" {
		t.Errorf("url = %q", r.URL)
	}
	if _, ok := r.Headers["X-Remote-User"]; ok {
		t.Error("X-Remote-User survived — that is the logged-in person's address")
	}
	if _, ok := r.Headers["Cookie"]; ok {
		t.Error("Cookie survived")
	}
	if r.Headers["User-Agent"] != "Mozilla/5.0" {
		t.Errorf("an allowed header was dropped: %v", r.Headers)
	}
}

func TestAnUnknownContextIsDropped(t *testing.T) {
	e := event()
	e.Contexts["runtime"] = sentry.Context{"name": "go"}
	e.Contexts["etwas_neues"] = sentry.Context{"wer": "jemand"}

	got := scrub.scrub(e, nil)

	if _, ok := got.Contexts["etwas_neues"]; ok {
		t.Error("an unknown context survived")
	}
	if _, ok := got.Contexts["runtime"]; !ok {
		t.Error("the runtime context was dropped")
	}
}

func TestSkipFieldDropsTheWholeEvent(t *testing.T) {
	e := event()
	e.Tags[SkipField] = "true"

	if got := scrub.scrub(e, nil); got != nil {
		t.Error("an event marked with SkipField was sent anyway")
	}
}

// The fingerprint is what keeps every log site from folding into one issue —
// and it must NOT be applied to events that carry a real stack trace.
func TestOnlyLogLinesAreFingerprintedOnTheCaller(t *testing.T) {
	logLine := event()
	logLine.Tags["caller"] = "web/app/jobrunner.go:82"

	if got := scrub.scrub(logLine, nil); len(got.Fingerprint) != 1 ||
		got.Fingerprint[0] != "web/app/jobrunner.go:82" {
		t.Errorf("fingerprint = %v", got.Fingerprint)
	}

	captured := sentry.NewEvent()
	captured.Tags["caller"] = "web/app/jobrunner.go:82"

	if got := scrub.scrub(captured, nil); len(got.Fingerprint) != 0 {
		t.Errorf("a captured event was given a caller fingerprint: %v", got.Fingerprint)
	}
}

func TestNoUserIsEverAttached(t *testing.T) {
	e := event()
	e.User = sentry.User{Email: "oliver.braun@hm.edu", Name: "Oliver Braun"}

	if got := scrub.scrub(e, nil); got.User.Email != "" || got.User.Name != "" {
		t.Errorf("a user survived: %+v", got.User)
	}
}

// An allowed tag whose VALUE carries an address must still be redacted — the
// allow list decides which keys survive, not what they may contain.
func TestAllowedTagValuesAreRedactedToo(t *testing.T) {
	e := event()
	e.Tags["caller"] = "gitlab/generate.go:111"
	e.Tags["panic"] = "runtime error on mpd-blatt01-alice_at_example.org"

	got := scrub.scrub(e, nil)

	if got.Tags["panic"] != "runtime error on [email]" {
		t.Errorf("panic tag = %q", got.Tags["panic"])
	}
}
