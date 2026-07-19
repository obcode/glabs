package mail

import (
	"bytes"
	"embed"
	htmltmpl "html/template"
	txttmpl "text/template"
	"time"

	blackfriday "github.com/russross/blackfriday/v2"
)

//go:embed tmpl/*.tmpl
var templates embed.FS

// Template file names for the four job-notification mails.
const (
	TmplScheduled = "jobScheduled.md.tmpl"
	TmplDone      = "jobDone.md.tmpl"
	TmplFailed    = "jobFailed.md.tmpl"
	TmplExpired   = "jobExpired.md.tmpl"
)

// JobMail is the data a job-notification template renders. It is library-neutral,
// so the runner does not depend on anything mail-internal.
type JobMail struct {
	Op         string
	Course     string
	Assignment string
	RunAt      time.Time
	GraceMin   int
	// Err is set for the failed/expired mails; Log carries the captured run output.
	Err string
	Log string
}

var funcs = txttmpl.FuncMap{
	// datetime renders a time in the server's local zone (Europe/Berlin, set in
	// main), which is what a human scheduled against.
	"datetime": func(t time.Time) string { return t.In(time.Local).Format("02.01.2006 15:04 MST") },
}

// Render renders the named Markdown template with data into the plain-text part
// (the Markdown itself, readable as-is, plus a footer) and the HTML part (Markdown
// → HTML wrapped in the shared base layout). One source feeds both, so they cannot
// drift. missingkey=error turns a template referencing an unknown field into an
// error instead of silently emitting "<no value>".
func Render(name string, data any) (text, html []byte, err error) {
	src, err := templates.ReadFile("tmpl/" + name)
	if err != nil {
		return nil, nil, err
	}
	tmpl, err := txttmpl.New(name).Funcs(funcs).Option("missingkey=error").Parse(string(src))
	if err != nil {
		return nil, nil, err
	}
	var md bytes.Buffer
	if err := tmpl.Execute(&md, data); err != nil {
		return nil, nil, err
	}

	text = assembleText(md.Bytes())
	// HardLineBreak: a single newline in the Markdown becomes <br>, so the HTML
	// keeps the line breaks the template author wrote.
	htmlBody := blackfriday.Run(md.Bytes(), blackfriday.WithExtensions(blackfriday.CommonExtensions|blackfriday.HardLineBreak))
	html, err = wrapHTML(htmlBody)
	return text, html, err
}

// assembleText appends the shared plain-text footer to a rendered Markdown body.
func assembleText(body []byte) []byte {
	var buf bytes.Buffer
	buf.Write(bytes.TrimRight(body, "\n"))
	buf.WriteString("\n\n--\nDiese E-Mail wurde automatisch von glabs erzeugt (https://glabs.cs.hm.edu).\n")
	return buf.Bytes()
}

// wrapHTML injects an already-rendered HTML fragment as the body of the shared
// base layout.
func wrapHTML(content []byte) ([]byte, error) {
	tmpl, err := htmltmpl.New("emailBaseHTML.tmpl").ParseFS(templates, "tmpl/emailBaseHTML.tmpl")
	if err != nil {
		return nil, err
	}
	if _, err := tmpl.New("content").Parse("{{ .Content }}"); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	data := struct{ Content htmltmpl.HTML }{Content: htmltmpl.HTML(content)} //nolint:gosec // our own rendered markdown
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
