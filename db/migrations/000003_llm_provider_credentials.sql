create table if not exists llm_credentials (
  id text primary key,
  owner_type text not null,
  guild_id text,
  user_id text,
  provider_id text not null,
  label text not null,
  credential_ciphertext bytea not null,
  credential_nonce bytea not null,
  credential_key_id text not null,
  credential_fingerprint text not null,
  status text not null default 'active',
  last_test_status text,
  last_tested_at timestamptz,
  last_error_code text,
  created_by_user_id text,
  updated_by_user_id text,
  revoked_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint llm_credentials_owner_type_check check (owner_type in ('guild', 'user', 'tenant')),
  constraint llm_credentials_owner_scope_check check (
    (owner_type = 'guild' and guild_id is not null and user_id is null) or
    (owner_type = 'user' and guild_id is null and user_id is not null) or
    (owner_type = 'tenant' and guild_id is null and user_id is null)
  ),
  constraint llm_credentials_status_check check (status in ('active', 'invalid', 'revoked')),
  constraint llm_credentials_last_test_status_check check (
    last_test_status is null or last_test_status in ('untested', 'succeeded', 'failed')
  ),
  unique (id, provider_id)
);

create unique index if not exists llm_credentials_active_owner_provider_label_idx
  on llm_credentials (
    owner_type,
    coalesce(guild_id, ''),
    coalesce(user_id, ''),
    provider_id,
    lower(label)
  )
  where revoked_at is null;

create index if not exists llm_credentials_active_provider_fingerprint_idx
  on llm_credentials (provider_id, credential_fingerprint)
  where revoked_at is null;

create index if not exists llm_credentials_owner_lookup_idx
  on llm_credentials (owner_type, guild_id, user_id, provider_id, status)
  where revoked_at is null;

create table if not exists llm_model_profiles (
  id text primary key,
  owner_type text not null,
  guild_id text,
  user_id text,
  purpose text not null,
  credential_id text not null,
  provider_id text not null,
  model_id text not null,
  params jsonb not null default '{}'::jsonb,
  enabled boolean not null default true,
  selected_by_user_id text,
  updated_at timestamptz not null default now(),
  constraint llm_model_profiles_owner_type_check check (owner_type in ('guild', 'user', 'tenant')),
  constraint llm_model_profiles_owner_scope_check check (
    (owner_type = 'guild' and guild_id is not null and user_id is null) or
    (owner_type = 'user' and guild_id is null and user_id is not null) or
    (owner_type = 'tenant' and guild_id is null and user_id is null)
  ),
  constraint llm_model_profiles_purpose_check check (purpose in ('chat', 'reasoning', 'embedding', 'routing')),
  foreign key (credential_id, provider_id) references llm_credentials(id, provider_id) on delete restrict
);

create unique index if not exists llm_model_profiles_enabled_owner_purpose_idx
  on llm_model_profiles (
    owner_type,
    coalesce(guild_id, ''),
    coalesce(user_id, ''),
    purpose
  )
  where enabled;

create index if not exists llm_model_profiles_credential_lookup_idx
  on llm_model_profiles (credential_id);

create index if not exists llm_model_profiles_provider_model_idx
  on llm_model_profiles (provider_id, model_id)
  where enabled;

create table if not exists llm_usage_events (
  id text primary key,
  request_id text not null,
  guild_id text,
  channel_id text,
  actor_user_id text not null,
  billing_owner_type text not null,
  billing_owner_id text not null,
  provider_id text not null,
  model_id text not null,
  purpose text not null,
  input_tokens integer not null default 0,
  output_tokens integer not null default 0,
  status text not null,
  error_class text,
  created_at timestamptz not null default now(),
  constraint llm_usage_events_billing_owner_type_check check (billing_owner_type in ('guild', 'user', 'tenant')),
  constraint llm_usage_events_purpose_check check (purpose in ('chat', 'reasoning', 'embedding', 'routing')),
  constraint llm_usage_events_token_count_check check (input_tokens >= 0 and output_tokens >= 0),
  constraint llm_usage_events_status_check check (status in ('succeeded', 'failed'))
);

create index if not exists llm_usage_events_request_idx
  on llm_usage_events (request_id);

create index if not exists llm_usage_events_billing_owner_created_idx
  on llm_usage_events (billing_owner_type, billing_owner_id, created_at desc);

create index if not exists llm_usage_events_guild_created_idx
  on llm_usage_events (guild_id, created_at desc)
  where guild_id is not null;

create index if not exists llm_usage_events_actor_created_idx
  on llm_usage_events (actor_user_id, created_at desc)
  where actor_user_id is not null;
