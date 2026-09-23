package migrate_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/obcode/glabs/v3/internal/mongotest"
	"github.com/obcode/glabs/v3/internal/pgtest"
	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/db/storetest"
	"github.com/obcode/glabs/v3/web/migrate"
	"github.com/obcode/glabs/v3/web/secrets"
	"github.com/spf13/viper"
)

// TestMain pins time.Local to Europe/Berlin, as cmd/glabs-web does for the
// server. The fixtures build timestamps from it, and the comparison between the
// two databases is only meaningful in the zone production runs in.
func TestMain(m *testing.M) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic("cannot load Europe/Berlin: " + err.Error())
	}
	time.Local = loc
	os.Exit(m.Run())
}

// TestImportCarriesEverythingAcross is the rehearsal of the cut-over, run on
// every push.
//
// It fills a MongoDB with the same shape of data production holds, runs the
// import exactly as the runbook does, and lets the tool's own verification
// decide. That verification is the strongest check available -- a full
// field-by-field read-back plus a decrypt of every token on both sides -- so a
// test that only re-implemented it would be weaker than just running it.
func TestImportCarriesEverythingAcross(t *testing.T) {
	mongo, mongoURI, mongoName := mongotest.NewDBWithURI(t)
	_, pgURI := pgtest.NewDBWithURI(t)
	ctx := t.Context()

	key := randomKey(t)
	sealer, err := secrets.NewSealer(key)
	if err != nil {
		t.Fatalf("cannot build the sealer: %v", err)
	}

	// --- a database that looks like production -----------------------------
	for _, owner := range []string{"a@hm.edu", "b@hm.edu"} {
		sealed, err := sealer.Seal("glpat-" + owner)
		if err != nil {
			t.Fatalf("cannot seal: %v", err)
		}
		if err := mongo.SaveUserGitLabToken(ctx, owner, sealed, storetest.SummerInstant()); err != nil {
			t.Fatalf("SaveUserGitLabToken: %v", err)
		}
		for _, name := range []string{"fopra", "sysprog"} {
			if err := mongo.SaveCourse(ctx, &db.StoredCourse{
				Owner: owner, Name: name, Source: storetest.FullCourseSource(),
				RawYAML:    []byte(name + ":\n  coursepath: fk07/" + name + " # kept verbatim\n"),
				ImportedAt: storetest.SummerInstant(), UpdatedAt: storetest.WinterInstant(),
			}); err != nil {
				t.Fatalf("SaveCourse: %v", err)
			}
			if err := mongo.RecordActivity(ctx, &db.ActivityEntry{
				Owner: owner, Course: name, Assignment: "blatt01", Op: "setaccess",
				Params: map[string]string{"accesslevel": "developer"},
				Status: "done", Detail: "12 Repositories", At: storetest.SummerInstant(),
			}); err != nil {
				t.Fatalf("RecordActivity: %v", err)
			}
			// One job with the optional fields set and one without, so the
			// nil-versus-empty distinction is part of the rehearsal.
			full := &db.ScheduledJob{
				Owner: owner, Op: "archive", Course: name, Assignment: "blatt01",
				OnlyFor: []string{"s1@hm.edu"}, Params: map[string]string{"x": "y"},
				RunAt: storetest.WinterInstant(), ConfigHash: "sha256:cafe",
				Status: db.JobPending, GraceMin: 30, CreatedAt: storetest.SummerInstant(),
			}
			bare := &db.ScheduledJob{
				Owner: owner, Op: "protect", Course: name, Assignment: "blatt02",
				RunAt: storetest.SummerInstant(), ConfigHash: "sha256:beef",
				Status: db.JobDone, GraceMin: 15, CreatedAt: storetest.SummerInstant(),
			}
			for _, j := range []*db.ScheduledJob{full, bare} {
				if err := mongo.SaveJob(ctx, j); err != nil {
					t.Fatalf("SaveJob: %v", err)
				}
			}
		}
		if err := mongo.RecordEvent(ctx, &db.Event{
			At: storetest.SummerInstant(), Type: db.EventLogin, Actor: owner,
			ActorName: "A. Beispiel", Department: "FK07", Severity: db.SeverityInfo,
		}); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}
	// A deleted token: the row exists, the token does not. It must not become a
	// row with half a token on the other side.
	if err := mongo.SaveUserGitLabToken(ctx, "c@hm.edu", mustSeal(t, sealer, "glpat-c"), storetest.SummerInstant()); err != nil {
		t.Fatalf("SaveUserGitLabToken: %v", err)
	}
	if err := mongo.DeleteUserGitLabToken(ctx, "c@hm.edu"); err != nil {
		t.Fatalf("DeleteUserGitLabToken: %v", err)
	}
	if err := mongo.SetSummarySentAt(ctx, storetest.WinterInstant()); err != nil {
		t.Fatalf("SetSummarySentAt: %v", err)
	}

	configure(t, mongoURI, mongoName, pgURI, key)

	// --- dry run writes nothing --------------------------------------------
	out := run(t, "--dry-run")
	if !strings.Contains(out, "nothing was written") {
		t.Errorf("--dry-run did not say so:\n%s", out)
	}
	pg, err := db.NewPG(ctx, pgURI)
	if err != nil {
		t.Fatalf("NewPG: %v", err)
	}
	defer pg.Disconnect(ctx) //nolint:errcheck
	counts, err := pg.CountsForImport(ctx)
	if err != nil {
		t.Fatalf("CountsForImport: %v", err)
	}
	if counts["courses"] != 0 || counts["events"] != 0 {
		t.Errorf("--dry-run wrote rows: %v", counts)
	}

	// --- the real thing -----------------------------------------------------
	out = run(t)
	// Logged, not just asserted on: this report is what a human reads at the
	// cut-over to decide whether to swap db.uri, so `go test -v` showing it makes
	// a change to it visible in review rather than only at 23:00 on the night.
	t.Log("\n" + out)
	if !strings.Contains(out, "verification passed") {
		t.Fatalf("the import did not verify:\n%s", out)
	}
	// The token check is the one that proves nobody has to re-enter a token, so
	// its absence must not pass for success.
	if !strings.Contains(out, "decrypted on both sides") {
		t.Errorf("the token check did not run:\n%s", out)
	}

	// --- spot checks the tool's own verification cannot make ----------------
	stored, err := pg.CourseOf(ctx, "a@hm.edu", "fopra")
	if err != nil {
		t.Fatalf("CourseOf: %v", err)
	}
	if diff := cmp.Diff(storetest.FullCourseSource(), stored.Source); diff != "" {
		t.Errorf("the course source changed on the way across (-want +got):\n%s", diff)
	}
	token, err := pg.GetUserSecret(ctx, "a@hm.edu")
	if err != nil {
		t.Fatalf("GetUserSecret: %v", err)
	}
	plaintext, err := sealer.Open(*token.GitLab)
	if err != nil {
		t.Fatalf("the migrated token does not decrypt: %v", err)
	}
	if plaintext != "glpat-a@hm.edu" {
		t.Errorf("the migrated token decrypts to something else")
	}
	deleted, err := pg.GetUserSecret(ctx, "c@hm.edu")
	if err != nil {
		t.Fatalf("GetUserSecret: %v", err)
	}
	if deleted != nil && deleted.GitLab != nil {
		t.Error("an owner whose token was deleted came across with a token")
	}

	// --- running it again ---------------------------------------------------
	// Without --reset it has to refuse rather than double every log entry:
	// activity and events have no natural key for an upsert to conflict on.
	if _, err := runErr("--dry-run=false"); err == nil {
		t.Error("a second run against a full database was accepted, want a refusal")
	}
	before, err := pg.CountsForImport(ctx)
	if err != nil {
		t.Fatalf("CountsForImport: %v", err)
	}
	out = run(t, "--reset")
	if !strings.Contains(out, "verification passed") {
		t.Fatalf("the repeated run did not verify:\n%s", out)
	}
	after, err := pg.CountsForImport(ctx)
	if err != nil {
		t.Fatalf("CountsForImport: %v", err)
	}
	// Same numbers, not doubled ones: --reset has to empty the tables, and the
	// activity and event rows are the ones that would otherwise accumulate.
	if diff := cmp.Diff(before, after); diff != "" {
		t.Errorf("the row counts changed on the repeated run (-before +after):\n%s", diff)
	}

	// --- verify-only changes nothing ---------------------------------------
	out = run(t, "--verify-only")
	if !strings.Contains(out, "verification passed") {
		t.Errorf("--verify-only failed on a database it had just filled:\n%s", out)
	}
}

// TestImportRunsBeforeTheConfigIsSwapped is the order the runbook actually
// prescribes, and the reason GLABS_PG_URI exists.
//
// At the cut-over the data is moved and verified FIRST and .glabs-web.yaml is
// swapped afterwards, so while the import runs, db.uri still names MongoDB.
// Taking the target from the config alone would force the opposite order and
// leave a window in which the config says PostgreSQL and PostgreSQL is empty.
func TestImportRunsBeforeTheConfigIsSwapped(t *testing.T) {
	mongo, mongoURI, mongoName := mongotest.NewDBWithURI(t)
	_, pgURI := pgtest.NewDBWithURI(t)
	ctx := t.Context()

	if err := mongo.SaveCourse(ctx, &db.StoredCourse{
		Owner: "a@hm.edu", Name: "fopra", Source: storetest.FullCourseSource(),
		ImportedAt: storetest.SummerInstant(), UpdatedAt: storetest.SummerInstant(),
	}); err != nil {
		t.Fatalf("SaveCourse: %v", err)
	}

	// db.uri as it really is that evening: still MongoDB.
	configure(t, mongoURI, mongoName, "mongodb://"+mongoURI, randomKey(t))

	// Without a target it has to refuse rather than write into MongoDB.
	if _, err := runErr(); err == nil {
		t.Error("a mongodb:// target was accepted")
	} else if !strings.Contains(err.Error(), migrate.EnvPGURI) {
		t.Errorf("error is %q, want it to name %s as the way out", err, migrate.EnvPGURI)
	}

	// With it, the import runs against the untouched config.
	t.Setenv(migrate.EnvPGURI, pgURI)
	out := run(t)
	if !strings.Contains(out, "verification passed") {
		t.Fatalf("the import did not verify:\n%s", out)
	}
}

func configure(t *testing.T, mongoURI, mongoName, pgURI, key string) {
	t.Helper()
	t.Setenv(migrate.EnvMongoURI, mongoURI)
	viper.Set("db.uri", pgURI)
	viper.Set("db.database", mongoName)
	viper.Set("secrets.key", key)
	t.Cleanup(func() {
		viper.Set("db.uri", "")
		viper.Set("db.database", "")
		viper.Set("secrets.key", "")
	})
}

func run(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runErr(args...)
	if err != nil {
		t.Fatalf("mongo2pg %v: %v\n%s", args, err, out)
	}
	return out
}

func runErr(args ...string) (string, error) {
	var buf bytes.Buffer
	err := migrate.Run(args, func() error { return nil }, &buf)
	return buf.String(), err
}

func mustSeal(t *testing.T, s *secrets.Sealer, plaintext string) secrets.SealedValue {
	t.Helper()
	v, err := s.Seal(plaintext)
	if err != nil {
		t.Fatalf("cannot seal: %v", err)
	}
	return v
}

// randomKey builds a valid 32-byte KEK. A fixed one would be a secret in the
// repository even though it protects nothing.
func randomKey(t *testing.T) string {
	t.Helper()
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("cannot generate a key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(b[:])
}
