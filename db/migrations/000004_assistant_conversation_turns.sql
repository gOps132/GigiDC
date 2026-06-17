create table if not exists assistant_conversation_turns (
  id text primary key,
  request_id text not null,
  surface text not null check (surface in ('dm', 'guild_mention')),
  guild_id text,
  channel_id text,
  actor_user_id text not null,
  role text not null check (role in ('user', 'assistant')),
  content_storage text not null default 'metadata_only' check (content_storage = 'metadata_only'),
  content_chars integer not null check (content_chars >= 0),
  provider_id text,
  model_id text,
  created_at timestamptz not null default now()
);

create index if not exists assistant_conversation_turns_guild_channel_idx
  on assistant_conversation_turns (guild_id, channel_id, created_at desc);

create index if not exists assistant_conversation_turns_request_idx
  on assistant_conversation_turns (request_id, created_at);

create index if not exists assistant_conversation_turns_actor_idx
  on assistant_conversation_turns (actor_user_id, created_at desc);
