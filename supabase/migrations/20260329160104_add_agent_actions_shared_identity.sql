create table if not exists agent_actions (
  id uuid primary key default gen_random_uuid(),
  guild_id text references guilds(id) on delete cascade,
  channel_id text,
  requester_user_id text not null,
  requester_username text not null,
  recipient_user_id text,
  recipient_username text,
  action_type text not null,
  status text not null default 'requested',
  visibility text not null default 'participants',
  title text not null,
  instructions text not null,
  result_summary text,
  error_message text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  completed_at timestamptz,
  constraint agent_actions_status_check check (
    status in ('requested', 'completed', 'failed', 'cancelled')
  ),
  constraint agent_actions_visibility_check check (
    visibility in ('requester_only', 'participants', 'guild')
  )
);

create index if not exists agent_actions_requester_created_at_idx
  on agent_actions (requester_user_id, created_at desc);

create index if not exists agent_actions_recipient_created_at_idx
  on agent_actions (recipient_user_id, created_at desc);

create index if not exists agent_actions_guild_created_at_idx
  on agent_actions (guild_id, created_at desc);

create index if not exists agent_actions_status_created_at_idx
  on agent_actions (status, created_at desc);
