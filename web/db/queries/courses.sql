-- Every read here takes an owner and filters on it. There is deliberately no
-- query that reads a course by name alone: owner isolation is enforced by the
-- shape of this file, not by remembering to add a filter at each call site.

-- name: CoursesOf :many
select * from courses
 where owner = $1
 order by name;

-- name: CourseOf :one
select * from courses
 where owner = $1 and name = $2;

-- name: SaveCourse :exec
-- An upsert on the key, replacing the whole row -- what ReplaceOne(upsert) did.
-- imported_at is included on purpose: the caller carries it on the struct and a
-- replace has to write it back, or editing a course would reset when it was
-- imported.
insert into courses (owner, name, source, raw_yaml, imported_at, updated_at)
values ($1, $2, $3, $4, $5, $6)
on conflict (owner, name) do update
   set source      = excluded.source,
       raw_yaml    = excluded.raw_yaml,
       imported_at = excluded.imported_at,
       updated_at  = excluded.updated_at;

-- name: DeleteCourse :execrows
-- :execrows so the caller can tell "deleted" from "there was nothing to
-- delete". Deleting another user's course must report not-found rather than
-- pretending it worked.
delete from courses
 where owner = $1 and name = $2;
