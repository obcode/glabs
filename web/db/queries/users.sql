-- name: GetUser :one
select * from users
 where email = $1;

-- name: ListUsers :many
-- Open requests first (they are what an admin comes here for), then everyone
-- else; newest request first within each group.
select * from users
 order by status = 'pending' desc, requested_at desc;

-- name: InsertAccessRequest :one
-- `do nothing` on a conflict, so a double click or a second tab cannot turn one
-- request into two admin mails: only the insert that actually created the row
-- gets a row back, the other gets pgx.ErrNoRows.
insert into users (email, name, department, status, reason, requested_at)
values ($1, $2, $3, 'pending', $4, $5)
on conflict (email) do nothing
returning *;

-- name: SetUserStatus :one
update users
   set status     = $2,
       decided_at = $3,
       decided_by = $4
 where email = $1
returning *;

-- name: DeleteUser :execrows
delete from users
 where email = $1;
