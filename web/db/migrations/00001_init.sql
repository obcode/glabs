-- The whole of glabs-web's persistence, ported from six MongoDB collections.
--
-- There are no joins anywhere in this schema and there is no foreign key between
-- the tables: the collections were independent in Mongo and the application
-- treats them that way. Introducing references now would be a design change
-- smuggled in under a data migration.
--
-- Ownership is the security property this layer enforces, and it is enforced by
-- the shape of the API above rather than by the database: `owner` is a plain
-- column, and every read that has one filters on it. See web/db/storetest, which
-- pins that for every implementation.
--
-- Every timestamp is timestamptz. PostgreSQL stores UTC and pgx decodes into
-- time.Local, which cmd/glabs-web/main.go sets to Europe/Berlin -- reproducing
-- exactly what SetBSONOptions(UseLocalTimeZone: true) did for the Mongo driver.
-- A plain `timestamp` would drop the offset and show every time in the digest
-- mail and the GUI one or two hours off, without anything failing.
--
-- timestamptz keeps MICROseconds, so a Go time.Time loses its last three digits
-- on the way through. That is not a regression: MongoDB stored milliseconds, so
-- this is three digits more than production has ever had. Nothing compares these
-- values for exact equality -- the digest windows are hours wide -- but do not
-- start: a timestamp that has been to the database and one that has not are not
-- the same value.

-- +goose Up

-- A course as imported by one user. (owner, name) IS the key -- it is the unique
-- index the Mongo version created, and the only access path web/db/courses.go
-- offers. No surrogate id: there is nothing to reference it by.
create table courses (
    owner       text        not null,
    name        text        not null,

    -- config.CourseSource as jsonb rather than ~20 normalised tables.
    --
    -- It is never filtered into: every query is keyed on (owner, name) and the
    -- document is always read and written whole. Normalising it would produce a
    -- table per sub-struct that no query touches, and would turn the most
    -- frequent change in the whole repository -- adding a field to
    -- config/source.go -- into a schema migration.
    --
    -- jsonb rather than json or text because it validates on write, drops
    -- duplicate keys, and leaves the door open for `source->>'coursepath'` or a
    -- GIN index later without a migration. What it does not preserve is key
    -- order and whitespace; that is what raw_yaml is for.
    source      jsonb       not null,

    -- The json tags on config.CourseSource ARE this column's format. Renaming one
    -- does not rename anything already stored -- it makes the old value
    -- unreadable and the field comes back as its zero value, silently. The
    -- version is checked on read so that becomes a hard failure instead.
    -- TestSourceTagsAreComplete guards the tags themselves.
    format_version int      not null default 1,

    -- The uploaded file, byte for byte, so a download returns exactly what was
    -- uploaded -- comments and key order included -- for as long as the course
    -- has not been edited through the web. bytea rather than text: the Go field
    -- is []byte and must not go through an encoding check. Null when the course
    -- was created in the GUI rather than uploaded; null and empty are different
    -- answers and stay different.
    raw_yaml    bytea,

    imported_at timestamptz not null,
    updated_at  timestamptz not null,

    primary key (owner, name)
);

-- A user's encrypted secrets. The plaintext never reaches the database; what is
-- stored is what secrets.SealedValue carries.
--
-- Three columns rather than one jsonb: it is a record with three fixed fields,
-- and bytea is the truthful type for a nonce and a ciphertext. jsonb would
-- base64 both and add a decode step on the most security-sensitive read path.
create table user_secrets (
    owner              text not null primary key,

    gitlab_key_version int,
    gitlab_nonce       bytea,
    gitlab_ciphertext  bytea,
    gitlab_updated_at  timestamptz,

    -- Makes "half a token" unrepresentable -- something Mongo's $set/$unset pair
    -- could never guarantee. Either all four are present or none are.
    constraint user_secrets_gitlab_all_or_nothing check (
        num_nonnulls(gitlab_key_version, gitlab_nonce, gitlab_ciphertext, gitlab_updated_at) in (0, 4)
    )
);

-- One mutating operation performed through the web against an assignment: the
-- web's stand-in for the shell history the CLI leaves behind.
create table activity (
    id         bigint generated always as identity primary key,
    owner      text        not null,
    course     text        not null,
    assignment text        not null,
    op         text        not null,

    -- Null, not '{}': absent parameters read back as a nil map, which is what
    -- `omitempty` produced in Mongo. An empty non-nil map has the same len() and
    -- a different answer to "were there any".
    params     jsonb,

    status     text        not null,
    detail     text        not null default '',
    at         timestamptz not null
);

-- The three indexes the Mongo version created, one per read. None is redundant:
-- (owner, course, at) is not a prefix of (owner, course, assignment, at).
create index activity_owner_course_assignment_at on activity (owner, course, assignment, at desc);
create index activity_owner_course_at            on activity (owner, course, at desc);
create index activity_owner_at                   on activity (owner, at desc);

-- One mutating operation queued to run at a wall-clock time. Jobs survive
-- restarts, missed runs are caught up after downtime, and the claim below keeps
-- two runners from firing the same job.
create table scheduled_jobs (
    -- text, not uuid: production is full of 24-hex ObjectId strings and the GUI
    -- passes them back and forth over GraphQL. text takes the old and the new.
    -- Generated in Go, as before, because SaveJob writes the id onto the caller's
    -- struct and the caller reads it afterwards.
    id            text        primary key,

    owner         text        not null,
    op            text        not null,
    course        text        not null,
    assignment    text        not null,

    -- text[] rather than jsonb: an ordered list, read and written whole, passed
    -- straight on as varargs. Null means "the whole assignment", which is not
    -- the same as "nobody" -- it decides which repositories an operation
    -- touches, so it must not be blurred into an empty array.
    only_for      text[],
    params        jsonb,

    run_at        timestamptz not null,

    -- Copied from the confirm token; the runner re-resolves and compares it at
    -- fire time, refusing a job whose config drifted since planning.
    config_hash   text        not null,

    status        text        not null,
    grace_minutes int         not null,
    created_at    timestamptz not null,
    started_at    timestamptz,
    finished_at   timestamptz,
    log           text        not null default '',
    err           text        not null default '',
    notified      boolean     not null default false,
    worker_id     text        not null default '',

    constraint scheduled_jobs_status check (
        status in ('pending', 'running', 'done', 'failed', 'expired', 'cancelled')
    )
);

-- Partial, and strictly better than Mongo's (status, runAt): the claim only ever
-- looks at pending jobs, so that is all the index holds.
create index scheduled_jobs_claim on scheduled_jobs (run_at) where status = 'pending';

-- The owner's list in the GUI, newest scheduled first.
create index scheduled_jobs_owner on scheduled_jobs (owner, run_at desc);

-- The notify sweep. In Mongo this read had no index at all and scanned every job
-- on each 30-second tick.
create index scheduled_jobs_notify on scheduled_jobs (finished_at)
    where notified = false and status in ('done', 'failed', 'expired');

-- The retention sweep. PostgreSQL has no TTL index; the runner deletes instead
-- (see ReapExpired), and `finished_at is null` falls out of the comparison on
-- its own, so pending and running jobs are never reaped.
create index scheduled_jobs_reap on scheduled_jobs (finished_at) where finished_at is not null;

-- Everything that happened on the platform worth an operator's attention.
--
-- Deliberately NOT owner-scoped: this is the one table that exists to be read
-- across all users by an admin. The optional string fields are `not null default
-- ''` rather than nullable -- Go cannot tell nil from "" for a string anyway, so
-- a nullable column would be a lie that costs scan code.
create table events (
    id         bigint      generated always as identity primary key,
    at         timestamptz not null,
    type       text        not null,
    severity   text        not null,
    actor      text        not null default '',
    actor_name text        not null default '',
    department text        not null default '',
    course     text        not null default '',
    assignment text        not null default '',
    op         text        not null default '',
    detail     text        not null default '',
    job_id     text        not null default ''
);

-- Both reads (the digest window and the admin feed) filter on `at` alone.
--
-- The id tiebreaker gives a stable total order that Mongo did not have: two
-- events in the same millisecond used to come back in arbitrary order. The
-- store contract suite therefore does not assert that order, so that it stays
-- green against both.
--
-- There is deliberately no (type, at) index. It existed in Mongo and no caller
-- ever used it: neither EventsBetween nor RecentEvents filters by type, and
-- summary.go groups in Go. An index nobody reads still costs every insert.
create index events_at on events (at desc, id desc);

-- Server-wide bookkeeping. Exactly one row, and the check makes that a property
-- of the schema rather than a convention -- a second row is impossible rather
-- than merely never written.
create table system_state (
    id              text primary key default 'state' check (id = 'state'),
    summary_sent_at timestamptz
);

-- Seeded here so SystemState() is a plain select that always finds a row. The
-- "never returns nil" contract still holds defensively in Go.
insert into system_state (id) values ('state') on conflict do nothing;

-- +goose Down

drop table system_state;
drop table events;
drop table scheduled_jobs;
drop table activity;
drop table user_secrets;
drop table courses;
