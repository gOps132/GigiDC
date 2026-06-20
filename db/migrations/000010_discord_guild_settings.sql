create table if not exists discord_guild_settings (
  guild_id text primary key,
  reply_latency_enabled boolean not null default false,
  updated_at timestamptz not null default now()
);
