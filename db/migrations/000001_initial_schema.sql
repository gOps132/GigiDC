create extension if not exists vector;

create table if not exists runtime_metadata (
  key text primary key,
  value text not null,
  updated_at timestamptz not null default now()
);

create table if not exists plugins (
  id text primary key,
  name text not null,
  source text not null,
  created_at timestamptz not null default now()
);

create table if not exists plugin_versions (
  id text primary key,
  plugin_id text not null references plugins(id) on delete cascade,
  version text not null,
  manifest jsonb not null,
  approved boolean not null default false,
  created_at timestamptz not null default now(),
  unique (plugin_id, version)
);

create table if not exists guild_plugin_installs (
  id text primary key,
  guild_id text not null,
  plugin_version_id text not null references plugin_versions(id) on delete restrict,
  enabled boolean not null default false,
  configured_by_user_id text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (guild_id, plugin_version_id)
);

create table if not exists guild_plugin_settings (
  guild_plugin_install_id text primary key references guild_plugin_installs(id) on delete cascade,
  settings jsonb not null default '{}'::jsonb,
  updated_at timestamptz not null default now()
);

create table if not exists jobs (
  id text primary key,
  kind text not null,
  payload jsonb not null default '{}'::jsonb,
  status text not null default 'queued',
  attempts integer not null default 0,
  run_after timestamptz not null default now(),
  locked_by text,
  locked_at timestamptz,
  last_error text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint jobs_status_check check (status in ('queued', 'running', 'succeeded', 'failed'))
);

create index if not exists jobs_claim_idx
  on jobs (status, run_after, created_at)
  where status = 'queued';

create table if not exists outbox_events (
  id text primary key,
  topic text not null,
  payload jsonb not null default '{}'::jsonb,
  status text not null default 'queued',
  attempts integer not null default 0,
  run_after timestamptz not null default now(),
  last_error text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint outbox_events_status_check check (status in ('queued', 'running', 'succeeded', 'failed'))
);

create index if not exists outbox_events_claim_idx
  on outbox_events (status, run_after, created_at)
  where status = 'queued';
