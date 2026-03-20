create table if not exists channel_ingestion_policies (
  id uuid primary key default gen_random_uuid(),
  guild_id text not null references guilds(id) on delete cascade,
  channel_id text not null,
  enabled boolean not null default false,
  updated_by_user_id text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (guild_id, channel_id)
);

create index if not exists channel_ingestion_policies_guild_idx
  on channel_ingestion_policies (guild_id);

create table if not exists clawbot_jobs (
  id uuid primary key default gen_random_uuid(),
  guild_id text not null references guilds(id) on delete cascade,
  channel_id text not null,
  thread_id text,
  requester_user_id text not null,
  command_name text not null,
  task_type text not null,
  status text not null default 'queued',
  clawbot_job_id text,
  request_payload jsonb not null default '{}'::jsonb,
  result_summary text,
  artifact_links jsonb not null default '[]'::jsonb,
  error_message text,
  result_posted_message_id text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  completed_at timestamptz,
  constraint clawbot_jobs_status_check check (
    status in ('queued', 'submitted', 'running', 'completed', 'failed', 'cancelled')
  )
);

create index if not exists clawbot_jobs_guild_idx
  on clawbot_jobs (guild_id, created_at desc);

create index if not exists clawbot_jobs_clawbot_job_idx
  on clawbot_jobs (clawbot_job_id);

create table if not exists audit_logs (
  id uuid primary key default gen_random_uuid(),
  guild_id text references guilds(id) on delete cascade,
  actor_user_id text,
  action text not null,
  target_type text not null,
  target_id text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists audit_logs_guild_created_at_idx
  on audit_logs (guild_id, created_at desc);
