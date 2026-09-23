-- name: SaveJob :exec
insert into scheduled_jobs (
    id, owner, op, course, assignment, only_for, params,
    run_at, config_hash, status, grace_minutes, created_at,
    started_at, finished_at, log, err, notified, worker_id
) values (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18
);

-- name: ClaimDueJob :one
-- The atomic claim. This single statement is what replaces a distributed lock:
-- exactly one runner ever owns a given job.
--
-- FOR UPDATE SKIP LOCKED is strictly stronger than the FindOneAndUpdate it
-- replaces -- several runners do not queue behind each other, they take
-- different jobs.
--
-- Deliberately NOT wrapped in an explicit transaction. The row lock ends with
-- the statement, and the flip to 'running' IS the lock. Holding a transaction
-- open across the job's execution would mean minutes with a locked row and a
-- pool connection tied up, which is the obvious-looking mistake here.
update scheduled_jobs
   set status = 'running', started_at = @now::timestamptz, worker_id = @worker_id
 where id = (
       select id from scheduled_jobs
        where status = 'pending' and run_at <= @now::timestamptz
        order by run_at
          for update skip locked
        limit 1)
returning *;

-- name: FinishJob :exec
-- Matching nothing is success, as before: the runner logs an error here, and a
-- job the retention sweep has already removed should not produce noise.
update scheduled_jobs
   set status = $2, finished_at = @finished_at::timestamptz, log = $3, err = $4
 where id = $1;

-- name: MarkNotified :exec
update scheduled_jobs
   set notified = true
 where id = $1;

-- name: CancelJob :one
-- Only one's own, only while still pending. A job already running is too late
-- and another owner's job is invisible: both return no row, and the caller turns
-- that into the same error. That sameness is deliberate -- "not found" must not
-- become an oracle for whether someone else has a job with that id.
update scheduled_jobs
   set status = 'cancelled', finished_at = @finished_at::timestamptz
 where id = $1 and owner = $2 and status = 'pending'
returning *;

-- name: JobsOf :many
-- The status list is optional: empty or null means no filter, which is what the
-- GUI's unfiltered view passes. Spelled with cardinality rather than a second
-- query so there is one ordering to keep correct.
select * from scheduled_jobs
 where owner = $1
   and (coalesce(cardinality(@statuses::text[]), 0) = 0 or status = any(@statuses::text[]))
 order by run_at desc, id desc;

-- name: JobOf :one
select * from scheduled_jobs
 where id = $1 and owner = $2;

-- name: UnnotifiedTerminalJobs :many
-- The runner's notify sweep, across all owners: one mail per finished job.
-- Cancelled is excluded -- the user did that on purpose and there is no mail for
-- it. This is what makes "mail on every terminal state" survive a crash between
-- finishing a job and mailing it.
select * from scheduled_jobs
 where notified = false
   and status in ('done', 'failed', 'expired')
 order by finished_at, id;

-- name: ReapExpiredJobs :execrows
-- The replacement for Mongo's TTL index on finished_at. `finished_at is null`
-- falls out of the comparison on its own, so pending and running jobs are never
-- reaped -- exactly what the partial TTL index did.
delete from scheduled_jobs
 where finished_at < @before::timestamptz;
