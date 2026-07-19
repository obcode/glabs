package mail

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"time"

	gomail "github.com/wneessen/go-mail"
)

// defaultMailHostname is the FQDN used for the HELO/EHLO greeting and the
// Message-ID domain when Config.Hostname is empty.
const defaultMailHostname = "glabs.cs.hm.edu"

// Sender sends rendered mails over SMTP. It is safe to build once and reuse.
type Sender struct {
	cfg Config
}

// NewSender builds a Sender for the given SMTP configuration.
func NewSender(cfg Config) *Sender { return &Sender{cfg: cfg} }

func (s *Sender) hostname() string {
	if s.cfg.Hostname != "" {
		return s.cfg.Hostname
	}
	return defaultMailHostname
}

// newClient builds an SMTP client with mandatory STARTTLS. Certificate
// verification follows the config (default: verify).
func (s *Sender) newClient() (*gomail.Client, error) {
	return gomail.NewClient(s.cfg.Host,
		gomail.WithPort(s.cfg.Port),
		gomail.WithHELO(s.hostname()),
		gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
		gomail.WithUsername(s.cfg.Username),
		gomail.WithPassword(s.cfg.Password),
		gomail.WithTLSPolicy(gomail.TLSMandatory),
		gomail.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: s.cfg.TLSInsecureSkipVerify, //nolint:gosec // configurable; default false (verify)
			ServerName:         s.cfg.Host,
		}),
	)
}

// buildMsg assembles a go-mail message with a plain-text body and an HTML
// alternative. It sets an explicit Message-ID with a real FQDN domain, since a
// Message-ID derived from a container's os.Hostname() is rejected by strict
// servers (554 5.7.1).
func (s *Sender) buildMsg(to, subject string, text, html []byte) (*gomail.Msg, error) {
	if s.cfg.From == "" {
		return nil, fmt.Errorf("no From address configured (smtp.from)")
	}
	msg := gomail.NewMsg()
	if err := msg.From(s.cfg.From); err != nil {
		return nil, fmt.Errorf("invalid From address %q: %w", s.cfg.From, err)
	}
	msg.SetMessageIDWithValue(newMessageID(s.hostname()))
	if err := msg.To(to); err != nil {
		return nil, fmt.Errorf("invalid To address %q: %w", to, err)
	}
	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextPlain, string(text))
	if len(html) > 0 {
		msg.AddAlternativeString(gomail.TypeTextHTML, string(html))
	}
	return msg, nil
}

// Send delivers a rendered mail. dryRun is a mandatory choice, never a default:
// when true the mail goes to the configured TestRecipient with a [DRY-RUN] subject
// prefix and never to the real recipient. A dry-run without a TestRecipient is an
// error rather than a silent no-op.
func (s *Sender) Send(dryRun bool, to, subject string, text, html []byte) error {
	if dryRun {
		if s.cfg.TestRecipient == "" {
			return fmt.Errorf("dry-run requested but smtp.testRecipient is not set")
		}
		to = s.cfg.TestRecipient
		subject = "[DRY-RUN] " + subject
	}
	msg, err := s.buildMsg(to, subject, text, html)
	if err != nil {
		return err
	}
	client, err := s.newClient()
	if err != nil {
		return err
	}
	return client.DialAndSend(msg)
}

// SendTest renders a representative job mail and sends it to the TestRecipient — a
// smoke test that the whole SMTP path works.
func (s *Sender) SendTest() error {
	if s.cfg.TestRecipient == "" {
		return fmt.Errorf("smtp.testRecipient is not set")
	}
	text, html, err := Render(TmplDone, SampleJob())
	if err != nil {
		return err
	}
	return s.Send(false, s.cfg.TestRecipient, "glabs SMTP-Test", text, html)
}

// SampleJob is representative data for SendTest and template tests.
func SampleJob() JobMail {
	return JobMail{
		Op:         "setaccess",
		Course:     "mpd",
		Assignment: "blatt01",
		RunAt:      time.Date(2026, 8, 8, 23, 59, 0, 0, time.UTC),
		GraceMin:   60,
		Err:        "",
		Log:        "running setaccess on mpd/blatt01 (41 repositories)\nsetaccess completed",
	}
}

// newMessageID builds an RFC 5322 Message-ID value ("random@host") with a
// cryptographically random local part.
func newMessageID(host string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read effectively never fails
	return hex.EncodeToString(b) + "@" + host
}
