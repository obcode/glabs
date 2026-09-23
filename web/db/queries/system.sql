-- name: SystemState :one
select * from system_state
 where id = 'state';

-- name: SetSummarySentAt :exec
-- Self-healing: the row is seeded by the migration, but an upsert means a
-- vanished row does not turn the nightly digest into a permanent failure.
insert into system_state (id, summary_sent_at)
values ('state', $1)
on conflict (id) do update
   set summary_sent_at = excluded.summary_sent_at;
