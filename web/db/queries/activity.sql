-- name: RecordActivity :exec
insert into activity (owner, course, assignment, op, params, status, detail, at)
values ($1, $2, $3, $4, $5, $6, $7, $8);

-- The three reads below all take a nullable limit, because `limit null` means
-- "no limit" in PostgreSQL -- exactly the `limit <= 0` case of the findActivity
-- helper they replace. The GUI reads pass the cap; the audit dump passes null.

-- name: ActivityFor :many
select * from activity
 where owner = $1 and course = $2 and assignment = $3
 order by at desc, id desc
 limit sqlc.narg('lim');

-- name: CourseActivityFor :many
select * from activity
 where owner = $1 and course = $2
 order by at desc, id desc
 limit sqlc.narg('lim');

-- name: AllActivityFor :many
select * from activity
 where owner = $1
 order by at desc, id desc
 limit sqlc.narg('lim');
