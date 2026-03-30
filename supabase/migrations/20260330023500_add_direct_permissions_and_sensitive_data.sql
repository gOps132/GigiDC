create table if not exists user_capability_grants (
  id uuid primary key default gen_random_uuid(),
  guild_id text not null references guilds(id) on delete cascade,
  user_id text not null,
  capability text not null,
  granted_by_user_id text not null,
  created_at timestamptz not null default now(),
  unique (guild_id, user_id, capability)
);

create index if not exists user_capability_grants_guild_user_idx
  on user_capability_grants (guild_id, user_id);

create table if not exists sensitive_data_records (
  id uuid primary key default gen_random_uuid(),
  guild_id text not null references guilds(id) on delete cascade,
  owner_user_id text not null,
  label text not null,
  description text,
  encrypted_value text not null,
  nonce text not null,
  created_by_user_id text not null,
  updated_by_user_id text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (guild_id, owner_user_id, label)
);

create index if not exists sensitive_data_records_guild_owner_idx
  on sensitive_data_records (guild_id, owner_user_id);
