create table if not exists guild_memory_policies (
  guild_id text primary key,
  default_retention_days integer not null default 90,
  raw_storage_mode text not null default 'metadata',
  embeddings_enabled boolean not null default false,
  updated_by_user_id text,
  updated_at timestamptz not null default now(),
  constraint guild_memory_policies_retention_check check (default_retention_days between 1 and 365),
  constraint guild_memory_policies_raw_storage_mode_check check (raw_storage_mode in ('off', 'metadata', 'full'))
);

create table if not exists guild_memory_channels (
  guild_id text not null,
  channel_id text not null,
  mode text not null default 'metadata',
  retention_days integer,
  updated_by_user_id text,
  updated_at timestamptz not null default now(),
  primary key (guild_id, channel_id),
  constraint guild_memory_channels_mode_check check (mode in ('off', 'metadata', 'full')),
  constraint guild_memory_channels_retention_check check (retention_days is null or retention_days between 1 and 365)
);

create index if not exists guild_memory_channels_mode_idx
  on guild_memory_channels (guild_id, mode);

create table if not exists guild_memory_messages (
  message_id text primary key,
  guild_id text not null,
  channel_id text not null,
  author_user_id text not null,
  normalized_text text,
  content_ciphertext bytea,
  content_hash text,
  created_at timestamptz not null,
  edited_at timestamptz,
  deleted_at timestamptz,
  retention_until timestamptz not null,
  indexed_at timestamptz not null default now(),
  constraint guild_memory_messages_content_check check (
    (normalized_text is not null and content_ciphertext is null) or
    (normalized_text is null and content_ciphertext is not null) or
    (normalized_text is null and content_ciphertext is null)
  )
);

create index if not exists guild_memory_messages_guild_channel_created_idx
  on guild_memory_messages (guild_id, channel_id, created_at desc);

create index if not exists guild_memory_messages_author_created_idx
  on guild_memory_messages (guild_id, author_user_id, created_at desc);

create index if not exists guild_memory_messages_retention_idx
  on guild_memory_messages (retention_until);

create table if not exists guild_memory_segments (
  id text primary key,
  message_id text not null references guild_memory_messages(message_id) on delete cascade,
  guild_id text not null,
  channel_id text not null,
  search_text text not null,
  token_count integer not null default 0,
  created_at timestamptz not null default now(),
  constraint guild_memory_segments_token_count_check check (token_count >= 0)
);

create index if not exists guild_memory_segments_guild_channel_created_idx
  on guild_memory_segments (guild_id, channel_id, created_at desc);

create table if not exists guild_memory_embeddings (
  segment_id text primary key references guild_memory_segments(id) on delete cascade,
  provider_id text not null,
  model_id text not null,
  embedding vector,
  embedded_at timestamptz not null default now()
);

create index if not exists guild_memory_embeddings_provider_model_idx
  on guild_memory_embeddings (provider_id, model_id);
