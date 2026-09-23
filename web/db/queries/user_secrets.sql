-- name: GetUserSecret :one
select * from user_secrets
 where owner = $1;

-- name: SaveUserGitLabToken :exec
-- The column list is what makes this touch only the gitlab fields: other
-- secrets on the row survive. In Mongo that was a $set of three named keys and
-- depended on remembering to name them.
insert into user_secrets (owner, gitlab_key_version, gitlab_nonce, gitlab_ciphertext, gitlab_updated_at)
values ($1, $2, $3, $4, $5)
on conflict (owner) do update
   set gitlab_key_version = excluded.gitlab_key_version,
       gitlab_nonce       = excluded.gitlab_nonce,
       gitlab_ciphertext  = excluded.gitlab_ciphertext,
       gitlab_updated_at  = excluded.gitlab_updated_at;

-- name: DeleteUserGitLabToken :exec
-- An update to null, not a delete of the row: the row is the anchor for the
-- other per-user secrets this table is meant to grow, and removing a GitLab
-- token must not take them with it. Exactly the $unset semantics.
--
-- A row that does not exist matches nothing and reports success, as before: the
-- caller's intent is "make sure there is none".
update user_secrets
   set gitlab_key_version = null,
       gitlab_nonce       = null,
       gitlab_ciphertext  = null,
       gitlab_updated_at  = null
 where owner = $1;
