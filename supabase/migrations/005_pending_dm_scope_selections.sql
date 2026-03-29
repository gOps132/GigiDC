create table if not exists pending_dm_scope_selections (
  id text primary key,
  user_id text not null,
  query text not null,
  scope_options jsonb not null default '[]'::jsonb,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null
);

create index if not exists pending_dm_scope_selections_user_expires_idx
  on pending_dm_scope_selections (user_id, expires_at desc);

create index if not exists pending_dm_scope_selections_expires_idx
  on pending_dm_scope_selections (expires_at);
