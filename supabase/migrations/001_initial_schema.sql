create extension if not exists pgcrypto;

create table if not exists guilds (
  id text primary key,
  name text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists role_policies (
  id uuid primary key default gen_random_uuid(),
  guild_id text not null references guilds(id) on delete cascade,
  capability text not null,
  discord_role_id text not null,
  created_at timestamptz not null default now(),
  unique (guild_id, capability, discord_role_id)
);

create index if not exists role_policies_guild_capability_idx
  on role_policies (guild_id, capability);

create table if not exists assignments (
  id uuid primary key default gen_random_uuid(),
  guild_id text not null references guilds(id) on delete cascade,
  title text not null,
  description text not null,
  due_at timestamptz,
  announcement_channel_id text,
  created_by_user_id text not null,
  published_message_id text,
  status text not null default 'draft',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint assignments_status_check check (status in ('draft', 'published'))
);

create index if not exists assignments_guild_created_at_idx
  on assignments (guild_id, created_at desc);

create index if not exists assignments_guild_status_idx
  on assignments (guild_id, status);
