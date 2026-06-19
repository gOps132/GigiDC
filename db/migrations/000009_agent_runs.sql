create table if not exists agent_runs (
  id text primary key,
  guild_id text,
  channel_id text,
  actor_user_id text,
  surface text not null,
  context_scope text,
  status text not null,
  termination_reason text,
  max_steps integer not null default 0,
  max_tool_calls integer not null default 0,
  max_llm_calls integer not null default 0,
  max_input_tokens integer not null default 0,
  max_output_tokens integer not null default 0,
  steps_used integer not null default 0,
  tool_calls_used integer not null default 0,
  llm_calls_used integer not null default 0,
  input_tokens_used integer not null default 0,
  output_tokens_used integer not null default 0,
  cancel_requested_at timestamptz,
  cancel_requested_by_user_id text,
  canceled_at timestamptz,
  completed_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint agent_runs_status_check check (status in ('running', 'succeeded', 'failed', 'denied', 'dry_run', 'confirmation_required', 'canceled'))
);

create index if not exists agent_runs_guild_created_idx
  on agent_runs (guild_id, created_at desc);

create index if not exists agent_runs_status_idx
  on agent_runs (status, created_at desc);

create table if not exists agent_run_steps (
  id bigserial primary key,
  run_id text not null references agent_runs(id) on delete cascade,
  step_index integer not null,
  kind text not null,
  status text not null,
  reason text,
  observation jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists agent_run_steps_run_idx
  on agent_run_steps (run_id, step_index, id);

create unique index if not exists agent_run_steps_run_step_idx
  on agent_run_steps (run_id, step_index);

create table if not exists agent_run_confirmations (
  id text primary key,
  run_id text not null references agent_runs(id) on delete cascade,
  step_index integer not null default 0,
  status text not null default 'pending',
  tool_name text,
  payload jsonb not null default '{}'::jsonb,
  created_by_user_id text,
  resolved_by_user_id text,
  expires_at timestamptz,
  confirmed_at timestamptz,
  canceled_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint agent_run_confirmations_status_check check (status in ('pending', 'confirmed', 'canceled', 'expired'))
);

create index if not exists agent_run_confirmations_pending_idx
  on agent_run_confirmations (run_id, expires_at)
  where status = 'pending';
