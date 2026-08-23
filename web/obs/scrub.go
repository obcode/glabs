// Package obs is the observability layer of glabs-web: it reports errors to a
// Sentry-compatible backend (GlitchTip) and owns the scrubber that decides what
// may leave this host.
//
// The scrubber is the security-relevant part. Read this file before adding
// anything that sends.
//
// It is the counterpart of plexams.go's obs package and tallox.go's
// internal/obs, and deliberately keeps their shape, so that a fix found in one
// repository reads the same in the others. Where it differs, the difference is
// marked and is a difference in what glabs HOLDS, not a difference of opinion.
//
// The one that matters: glabs-web is not a self-contained service. It imports
// the shared config, gitlab, gitlab/report and reporter packages — the same code
// the CLI runs — and those log at Error level with fields naming students:
// email, student, user, name, project, projectPath, group, groupPath. Every one
// of those lines reaches this scrubber. tallox's allow list would have let some
// through, which is why this one was built from what glabs logs rather than
// copied.
package obs

import (
	"net/url"
	"regexp"
	"strings"

	sentry "github.com/getsentry/sentry-go"
)

// zerologLogger is the value sentryzerolog stamps on every event it builds. It
// is how the scrubber tells a log line apart from an event captured elsewhere:
// only the former needs the caller fingerprint, because only the former has a
// useless stack trace.
const zerologLogger = "zerolog"

// SkipField set on a zerolog line drops that line from the error report while
// leaving it in the local log:
//
//	log.Error().Err(err).Bool(obs.SkipField, true).Msg("...")
//
// Reach for sentry.ignoreerrors in the configuration first — this one needs a
// deploy. It exists for the single call site that is known noise.
const SkipField = "sentry_skip"

// allowedTags is a POSITIVE list, and it is the whole point of this file.
//
// sentryzerolog turns EVERY unknown zerolog field into a Sentry tag, unfiltered.
// A deny list would have to be kept in step with every log line anyone ever
// writes; this direction is safe by itself — a field nobody thought about is
// dropped, not sent.
//
// What is here is the complete field vocabulary of web/ as of 2026-08-23, which
// is impersonal throughout, plus the fingerprint key and the one domain term
// that names a course rather than a person.
//
// Deliberately NOT here, and each for a reason:
//
//	email, student, user, name  — say what they are.
//	project, projectPath        — a generated project is named <assignment>-<student>
//	                              (docs/configuration.md), so it NAMES A PERSON despite
//	                              sounding structural. This is the trap in this repo.
//	group, groupPath            — ambiguous: some sites pass the course, others a group
//	                              path that carries members. Ambiguous loses.
//	url                         — a GitLab URL contains the project path, see above.
//	searchPattern, info, issue, branch, rule
//	                            — free text or unaudited; nothing needs them yet.
//
// Add a key only after checking that it cannot carry a student's identity.
var allowedTags = map[string]bool{
	"caller":     true, // the fingerprint key, and the join key back to the logs
	"job":        true, // opaque scheduled-job id
	"worker":     true,
	"type":       true,
	"port":       true,
	"host":       true,
	"interval":   true,
	"hour":       true,
	"production": true,
	"dryRun":     true,
	"panic":      true, // the runtime panic value; redacted like any other string
	"course":     true, // the COURSE, never who is in it
}

// allowedHeaders are the request headers worth keeping. Same direction as the
// tags, and for a sharper reason: X-Remote-User carries the logged-in person's
// mail address, and Cookie carries their session.
var allowedHeaders = map[string]bool{
	"User-Agent":   true,
	"Content-Type": true,
	"Accept":       true,
}

// allowedContexts are the SDK's own runtime contexts. Nothing in glabs-web sets
// a context, so anything else appearing here is unaccounted for and goes.
var allowedContexts = map[string]bool{
	"device":  true,
	"os":      true,
	"runtime": true,
	"trace":   true,
}

// reEmail matches the ordinary spelling of the one personal identifier that
// turns up in free text — a message, an error string, a URL — where no allow
// list can help.
//
// DIFFERENCE FROM plexams: no Matrikelnummer pattern. plexams redacts runs of
// 7 to 10 digits because it handles student registrations; glabs identifies
// students by GitLab username or mail address, and the same rule here would only
// eat GitLab ids and durations out of otherwise readable messages.
var reEmail = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)

// reAtScheme is the glabs-specific half, and it exists because the ordinary one
// does not fire here.
//
// glabs generates one project per student named <assignment>-<student>, and when
// students are identified by mail address the '@' is replaced by '_at_' for
// filesystem compatibility (docs/configuration.md): mpd-blatt01-alice_at_example.org.
// That string contains no '@' and sails straight through reEmail — while being
// exactly as identifying as the address it was made from. GitLab error responses
// quote these paths back verbatim.
//
// It matches GREEDILY to the left, and that is deliberate rather than sloppy:
// mpd-blatt01-alice_at_example.org is redacted whole, assignment prefix and all.
// A tighter pattern that kept the prefix cannot exist, because a student whose
// name carries a hyphen (mpd-blatt01-anna-lena_at_hm.edu) leaves no character at
// which the assignment ends and the person begins — it would print
// "mpd-blatt01-anna-[email]" and hand out half the name. The assignment is
// recoverable from the `course` tag and from the message; half a name is not
// recoverable from anything, and must not be sent.
//
// KNOWN RESIDUAL RISK, decided deliberately on 2026-08-23: a student identified
// by a bare GitLab username (mpd-blatt01-alice) is not matched by either
// pattern. Catching it would mean redacting anything shaped like <word>-<word>,
// which eats the assignment and course names that make a report worth reading.
// The allow list above is what carries the weight there; this is the second line
// of defence, not the first.
var reAtScheme = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+_at_[a-z0-9.\-]+\.[a-z]{2,}`)

func redact(s string) string {
	if s == "" {
		return s
	}
	s = reEmail.ReplaceAllString(s, "[email]")
	return reAtScheme.ReplaceAllString(s, "[email]")
}

// scrubber is the BeforeSend hook. Its zero value is the safe configuration, so
// a test can use scrubber{} and get production behaviour.
type scrubber struct{}

// scrub is what every event passes through on its way out. It is fail-closed
// throughout: it copies the few things that are allowed into fresh containers
// rather than deleting the things it knows are bad, so a field, header or
// context nobody anticipated is dropped by default.
func (s scrubber) scrub(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event == nil {
		return nil
	}

	// The emergency exit, honoured before anything else.
	if _, skip := event.Tags[SkipField]; skip {
		return nil
	}

	// Group log lines by their call site. sentryzerolog builds the stack trace
	// INSIDE its Write method, so the top frames are identical zerolog internals
	// for every log site and the default grouping would fold unrelated failures
	// into one issue. The caller field points at the failing line instead.
	//
	// Events captured elsewhere keep the default grouping — their stacks are real.
	if caller := event.Tags["caller"]; caller != "" && event.Logger == zerologLogger {
		event.Fingerprint = []string{caller}
	}

	event.Tags = allowMap(event.Tags, allowedTags)
	event.Message = redact(event.Message)
	event.Transaction = redact(event.Transaction)

	for i := range event.Exception {
		event.Exception[i].Type = redact(event.Exception[i].Type)
		event.Exception[i].Value = redact(event.Exception[i].Value)
	}

	for _, b := range event.Breadcrumbs {
		if b == nil {
			continue
		}
		b.Message = redact(b.Message)
		b.Data = allowData(b.Data)
	}

	// Rebuild rather than prune: query string, POST body, cookies and the CGI
	// environment all go, and the headers survive only by name.
	if r := event.Request; r != nil {
		event.Request = &sentry.Request{
			Method:  r.Method,
			URL:     redactURL(r.URL),
			Headers: allowMap(r.Headers, allowedHeaders),
		}
	}

	event.Contexts = allowContexts(event.Contexts)

	// No user is ever attached. DIFFERENCE FROM plexams: that one keys a stable
	// pseudonym with its existing secrets.key, which buys "3 people affected"
	// without sending an address. glabs-web has a secrets.key too, but it seals
	// GitLab tokens; borrowing it to carry student identity out of this host is
	// not a thing to do in passing. Until there is a decision, nobody is named.
	event.User = sentry.User{}

	return event
}

func allowMap(in map[string]string, allowed map[string]bool) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if allowed[k] {
			out[k] = redact(v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func allowData(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if !allowedTags[k] {
			continue
		}
		if s, ok := v.(string); ok {
			out[k] = redact(s)
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func allowContexts(in map[string]sentry.Context) map[string]sentry.Context {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]sentry.Context, len(in))
	for k, v := range in {
		if allowedContexts[k] {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// redactURL keeps scheme, host and path and throws the query away wholesale —
// a filter parameter is exactly where a name would show up.
func redactURL(raw string) string {
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		// Unparsable: fall back to cutting at the first '?' and redacting the
		// rest, rather than passing it through.
		if i := strings.IndexByte(raw, '?'); i >= 0 {
			return redact(raw[:i])
		}
		return redact(raw)
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	return redact(u.String())
}

// allowedHeaderTerms is allowedHeaders as the SDK wants it, kept derived so the
// two cannot drift apart.
func allowedHeaderTerms() []string {
	terms := make([]string, 0, len(allowedHeaders))
	for h := range allowedHeaders {
		terms = append(terms, h)
	}
	return terms
}
