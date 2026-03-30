alter table agent_actions
  add column if not exists confirmation_requested_at timestamptz,
  add column if not exists confirmation_expires_at timestamptz,
  add column if not exists confirmed_at timestamptz,
  add column if not exists confirmed_by_user_id text,
  add column if not exists cancelled_at timestamptz;

alter table agent_actions
  drop constraint if exists agent_actions_status_check;

alter table agent_actions
  add constraint agent_actions_status_check check (
    status in ('requested', 'awaiting_confirmation', 'in_progress', 'completed', 'failed', 'cancelled')
  );

create index if not exists agent_actions_pending_confirmation_idx
  on agent_actions (requester_user_id, confirmation_expires_at asc, created_at desc)
  where status = 'awaiting_confirmation';

create table if not exists user_profiles (
  guild_id text not null references guilds(id) on delete cascade,
  user_id text not null,
  username text not null,
  global_name text,
  display_name text,
  first_seen_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now(),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (guild_id, user_id)
);

create index if not exists user_profiles_last_seen_idx
  on user_profiles (guild_id, last_seen_at desc);

create table if not exists user_memory_snapshots (
  id uuid primary key default gen_random_uuid(),
  guild_id text not null references guilds(id) on delete cascade,
  user_id text not null,
  snapshot_kind text not null,
  summary_text text not null,
  source_message_ids text[] not null default '{}'::text[],
  source_action_ids text[] not null default '{}'::text[],
  generated_at timestamptz not null default now(),
  expires_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint user_memory_snapshots_kind_check check (
    snapshot_kind in ('identity_summary', 'working_context', 'preferences')
  ),
  constraint user_memory_snapshots_user_kind_unique unique (guild_id, user_id, snapshot_kind)
);

create index if not exists user_memory_snapshots_user_generated_idx
  on user_memory_snapshots (guild_id, user_id, generated_at desc);
