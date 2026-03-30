create table if not exists pending_dm_recipient_selections (
  id uuid primary key,
  requester_user_id text not null,
  requester_username text not null,
  relay_message text not null,
  relay_context text,
  recipient_options jsonb not null,
  guild_id text,
  channel_id text not null,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null
);

create index if not exists pending_dm_recipient_selections_user_expires_idx
  on pending_dm_recipient_selections (requester_user_id, expires_at desc);

create index if not exists pending_dm_recipient_selections_expires_idx
  on pending_dm_recipient_selections (expires_at);
