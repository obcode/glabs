-- +goose Up

-- Who may use glabs-web. The auth proxy only says who someone is; this table
-- says whether they are let in. A row exists from the moment someone asks for
-- access, and its status moves on when an admin decides.
--
-- No row at all means "never asked" -- deliberately not a status value, so
-- resetting a decision is a delete and asking again is an insert, and neither
-- needs a special case. Admins from the server config never need a row: the app
-- lets them in before it looks here.
--
-- Only additive, so the previous binary runs unchanged against this schema.
create table users (
    email        text        primary key, -- lowercased, as the auth middleware delivers it
    name         text        not null default '',
    department   text        not null default '',
    status       text        not null check (status in ('pending', 'approved', 'rejected', 'revoked')),
    reason       text        not null default '', -- what the requester wrote; length is checked in the app
    requested_at timestamptz not null,
    decided_at   timestamptz,
    decided_by   text        not null default ''
);

-- The admin list shows open requests first and filters by status.
create index users_status on users (status, requested_at);

-- +goose Down

drop table users;
