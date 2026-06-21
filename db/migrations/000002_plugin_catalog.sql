alter table plugins
  add column if not exists source_kind text not null default 'known',
  add column if not exists discord_application_id text,
  add column if not exists discord_bot_user_id text,
  add column if not exists manifest_url text,
  add column if not exists updated_at timestamptz not null default now();

create unique index if not exists plugins_discord_application_id_idx
  on plugins (discord_application_id)
  where discord_application_id is not null and discord_application_id <> '';

create unique index if not exists plugins_discord_bot_user_id_idx
  on plugins (discord_bot_user_id)
  where discord_bot_user_id is not null and discord_bot_user_id <> '';

alter table plugin_versions
  add column if not exists manifest_sha256 text,
  add column if not exists source_url text,
  add column if not exists approved_by_user_id text,
  add column if not exists public_dispatch_allowed boolean not null default false;

create index if not exists plugin_versions_approved_lookup_idx
  on plugin_versions (plugin_id, approved, version);

alter table guild_plugin_installs
  add column if not exists enabled_by_user_id text;

create index if not exists guild_plugin_installs_enabled_lookup_idx
  on guild_plugin_installs (guild_id, enabled, plugin_version_id);
