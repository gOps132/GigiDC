create table if not exists llm_guild_policies (
  guild_id text primary key,
  personal_keys_mode text not null default 'off' check (personal_keys_mode in ('off', 'dm-only', 'guild-allowed')),
  updated_by_user_id text,
  updated_at timestamptz not null default now()
);
