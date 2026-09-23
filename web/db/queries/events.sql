-- name: RecordEvent :exec
insert into events (at, type, severity, actor, actor_name, department, course, assignment, op, detail, job_id)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: EventsBetween :many
-- [since, until) -- half open on purpose. The upper bound belongs to the NEXT
-- digest window; including it would mail the same event twice.
--
-- Oldest first, with the id as tiebreaker. Mongo left events sharing a timestamp
-- in an arbitrary order; this is a stable total order and a free improvement.
select * from events
 where at >= $1 and at < $2
 order by at, id;

-- name: RecentEvents :many
-- The admin page's live feed. A null limit means unlimited, as with activity.
select * from events
 where at >= $1
 order by at desc, id desc
 limit sqlc.narg('lim');

-- name: ReapExpiredEvents :execrows
-- The replacement for Mongo's TTL index on at: the monitoring trail is a rolling
-- window, not a permanent archive.
delete from events
 where at < @before::timestamptz;
