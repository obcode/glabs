package migrate

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/secrets"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnvMongoURI and EnvPGURI are where the two connection strings come from.
//
// Environment variables rather than flags, and the same rule the backup scripts
// follow: `ps` shows every argv on this host to every user, and both of these
// carry a database root password.
//
// EnvPGURI exists because of the ORDER of the cut-over: the import runs while
// .glabs-web.yaml still points at MongoDB, because the config is only swapped
// once the data is across and verified. Without a way to name the target
// separately, the tool would have to be run after the swap -- leaving a window
// in which the config says PostgreSQL and PostgreSQL is empty. It falls back to
// db.uri, which is what makes --verify-only convenient after the swap.
const (
	EnvMongoURI = "GLABS_MONGO_URI"
	EnvPGURI    = "GLABS_PG_URI"
)

// Options is what the operator asked for.
type Options struct {
	DryRun     bool
	VerifyOnly bool
	Reset      bool
}

// Run is the `glabs-web mongo2pg` subcommand. args are the arguments after the
// subcommand name.
//
// The PostgreSQL URI and the secrets key come from the config file that is
// already mounted into the container, so the only thing to pass in is the
// MongoDB URI.
func Run(args []string, loadConfig func() error, out io.Writer) error {
	var o Options
	fs := flag.NewFlagSet("mongo2pg", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.BoolVar(&o.DryRun, "dry-run", false, "read and check everything, write nothing")
	fs.BoolVar(&o.VerifyOnly, "verify-only", false, "compare both databases, change nothing")
	fs.BoolVar(&o.Reset, "reset", false, "empty the target tables first (for a repeated run)")
	fs.Usage = func() {
		printf(out, `usage: glabs-web mongo2pg [flags]

Imports glabs-web's data from MongoDB into PostgreSQL, once.

  %s must hold the MongoDB connection string.
  %s holds the PostgreSQL one; without it, db.uri from .glabs-web.yaml is used.
  The encryption key (secrets.key) always comes from .glabs-web.yaml.

flags:
`, EnvMongoURI, EnvPGURI)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		// Asking for help is not a failure. Without this the usage text is
		// followed by "Error: flag: help requested" and a non-zero exit.
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if o.DryRun && o.VerifyOnly {
		return errors.New("--dry-run and --verify-only contradict each other")
	}

	if err := loadConfig(); err != nil {
		return err
	}

	mongoURI := os.Getenv(EnvMongoURI)
	if mongoURI == "" {
		return fmt.Errorf("%s is not set -- it must hold the MongoDB connection string", EnvMongoURI)
	}
	pgURI := os.Getenv(EnvPGURI)
	source := EnvPGURI
	if pgURI == "" {
		pgURI, source = viper.GetString("db.uri"), "db.uri"
	}
	if pgURI == "" {
		return fmt.Errorf("no PostgreSQL target: set %s, or point db.uri at PostgreSQL", EnvPGURI)
	}
	if db.LooksLikeMongoURI(pgURI) {
		return fmt.Errorf("the PostgreSQL target taken from %s points at MongoDB (%s). "+
			"During the cut-over db.uri is still the MongoDB one, so pass the PostgreSQL URI in %s",
			source, redactURI(pgURI), EnvPGURI)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	client, err := mongo.Connect(options.Client().
		ApplyURI(mongoURI).
		SetBSONOptions(&options.BSONOptions{UseLocalTimeZone: true}))
	if err != nil {
		return fmt.Errorf("cannot connect to mongodb: %w", err)
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	// UseLocalTimeZone exactly as the server sets it, so a timestamp arrives in
	// the same location the running application would have read it in. Without
	// it every instant would still be correct and every rendering an hour off.

	pg, err := db.NewPG(ctx, pgURI)
	if err != nil {
		return err
	}
	defer pg.Disconnect(ctx) //nolint:errcheck

	// The schema is brought up first, so the target may be a completely empty
	// database -- which is what it is on the evening it is created.
	if !o.DryRun {
		if err := pg.MigrateSchema(ctx); err != nil {
			return err
		}
	}

	// The MongoDB database name: from db.database while the config still describes
	// MongoDB, and "glabs" otherwise -- which is both the production name and the
	// default the server itself falls back to.
	mongoDB := viper.GetString("db.database")
	if mongoDB == "" {
		mongoDB = "glabs"
	}
	printf(out, "reading from mongodb %s, database %s\n", redactURI(mongoURI), mongoDB)
	src, err := readAll(ctx, client, mongoDB)
	if err != nil {
		return err
	}
	printCounts(out, "read from mongodb", src.Counts())

	if o.DryRun {
		println(out, "\n--dry-run: nothing was written.")
		println(out, "Every document above decoded into the application's own types;")
		println(out, "a document that could not be read would have failed this run.")
		return nil
	}

	if !o.VerifyOnly {
		if err := write(ctx, out, pg, src, o.Reset); err != nil {
			return err
		}
	}

	return verify(ctx, out, pg, src, sealerFor(out))
}

// write loads the source into PostgreSQL.
//
// The order is the one in db.ImportTables: the irreplaceable first, so a run
// that dies half way has already carried across the part nobody can recreate.
func write(ctx context.Context, out io.Writer, pg *db.PG, src *source, reset bool) error {
	counts, err := pg.CountsForImport(ctx)
	if err != nil {
		return err
	}
	if total(counts) > 0 {
		if !reset {
			printCounts(out, "already in postgresql", counts)
			return errors.New("the target is not empty. Re-run with --reset to replace its contents, " +
				"or point db.uri at the right database")
		}
		println(out, "--reset: emptying the target tables")
		if err := pg.TruncateAllForImport(ctx); err != nil {
			return err
		}
	}

	if src.State != nil && src.State.SummarySentAt != nil {
		if err := pg.SetSummarySentAt(ctx, *src.State.SummarySentAt); err != nil {
			return err
		}
	}

	// The sealed token is written back exactly as it was stored: this tool never
	// decrypts anything on the way through. Re-encrypting would mean a nonce
	// change for no reason, and a bug in it would be unrecoverable.
	for _, s := range src.Secrets {
		if s.GitLab == nil {
			// A user who once had a token and removed it: MongoDB's delete unsets
			// the fields and keeps the document. Carried across as an empty row
			// rather than dropped -- see EnsureSecretsRowForImport.
			if err := pg.EnsureSecretsRowForImport(ctx, s.Owner); err != nil {
				return err
			}
			continue
		}
		updatedAt := time.Time{}
		if s.GitLabUpdatedAt != nil {
			updatedAt = *s.GitLabUpdatedAt
		}
		if err := pg.SaveUserGitLabToken(ctx, s.Owner, *s.GitLab, updatedAt); err != nil {
			return fmt.Errorf("cannot write the token of %s: %w", s.Owner, err)
		}
	}

	for _, c := range src.Courses {
		if err := pg.SaveCourse(ctx, c); err != nil {
			return fmt.Errorf("cannot write course %s/%s: %w", c.Owner, c.Name, err)
		}
	}
	for _, j := range src.Jobs {
		if err := pg.SaveJob(ctx, j); err != nil {
			return fmt.Errorf("cannot write job %s: %w", j.ID, err)
		}
	}
	for _, a := range src.Activity {
		if err := pg.RecordActivity(ctx, a); err != nil {
			return fmt.Errorf("cannot write an activity entry of %s: %w", a.Owner, err)
		}
	}
	for _, e := range src.Events {
		if err := pg.RecordEvent(ctx, e); err != nil {
			return fmt.Errorf("cannot write an event: %w", err)
		}
	}

	written, err := pg.CountsForImport(ctx)
	if err != nil {
		return err
	}
	printCounts(out, "written to postgresql", written)
	return nil
}

// sealerFor builds the sealer for the token check, or nil when no key is
// configured. Without it the import still works -- the ciphertext is copied
// verbatim either way -- only the strongest check is skipped, and the report
// says so rather than quietly passing.
func sealerFor(out io.Writer) *secrets.Sealer {
	key := viper.GetString("secrets.key")
	if key == "" {
		println(out, "note: secrets.key is not configured -- the token check will be skipped")
		return nil
	}
	sealer, err := secrets.NewSealer(key)
	if err != nil {
		printf(out, "note: secrets.key cannot be used (%v) -- the token check will be skipped\n", err)
		return nil
	}
	return sealer
}

func total(counts map[string]int64) int64 {
	var n int64
	for table, c := range counts {
		// The seeded system_state row is there before any import and does not
		// make the target "not empty".
		if table == "system_state" {
			continue
		}
		n += c
	}
	return n
}

func printCounts(out io.Writer, title string, counts map[string]int64) {
	printf(out, "\n%s:\n", title)
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	tables := make([]string, 0, len(counts))
	for t := range counts {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		printf(tw, "  %s\t%d\n", t, counts[t])
	}
	tw.Flush() //nolint:errcheck
}

// redactURI keeps a connection string printable: host and port, nothing else.
// These strings carry database passwords, and this output goes into a terminal,
// a log file and quite possibly a paste into a chat.
func redactURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Host == "" {
		return "the configured server"
	}
	return u.Host
}
