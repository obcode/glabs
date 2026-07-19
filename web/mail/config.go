// Package mail sends glabs-web's job-notification emails. It keeps the go-mail
// dependency and the Markdown→text+HTML rendering confined here, so callers deal
// only with a small library-neutral surface (Config, Sender, Render, JobMail).
//
// Every notification renders from a single Markdown template into both a text and
// an HTML part, so the two can never drift. SMTP credentials live in the server
// config file, never in the database.
package mail

// Config is the SMTP configuration, read from the server config in bootstrap. It
// is passed to NewSender; the mail package itself never touches viper.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	// From is the address mails are sent as (the From header and, by default, the
	// SMTP envelope sender). Required.
	From string
	// Hostname is the FQDN used for the SMTP HELO/EHLO greeting and the Message-ID
	// domain. Empty falls back to defaultMailHostname; strict servers reject a
	// Message-ID derived from a container's os.Hostname().
	Hostname string
	// TLSInsecureSkipVerify disables server-certificate verification. Unlike
	// plexams (which hard-codes skip), this defaults to false — verify.
	TLSInsecureSkipVerify bool
	// TestRecipient receives dry-run sends and SendTest smoke tests.
	TestRecipient string
}
