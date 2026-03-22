create extension if not exists vector;
create extension if not exists pg_trgm;

alter table assignments
  add column if not exists mentioned_role_ids jsonb not null default '[]'::jsonb;

create table if not exists messages (
  id text primary key,
  guild_id text references guilds(id) on delete cascade,
  channel_id text not null,
  thread_id text,
  dm_user_id text,
  author_user_id text not null,
  author_username text not null,
  author_is_bot boolean not null default false,
  content text not null default '',
  attachment_count integer not null default 0,
  created_at timestamptz not null,
  edited_at timestamptz,
  indexed_at timestamptz not null default now(),
  constraint messages_scope_check check (
    (guild_id is not null and dm_user_id is null)
    or (guild_id is null and dm_user_id is not null)
  )
);

create index if not exists messages_dm_user_created_at_idx
  on messages (dm_user_id, created_at desc)
  where guild_id is null;

create index if not exists messages_guild_created_at_idx
  on messages (guild_id, created_at desc)
  where guild_id is not null;

create index if not exists messages_channel_created_at_idx
  on messages (channel_id, created_at desc);

create index if not exists messages_content_trgm_idx
  on messages using gin (content gin_trgm_ops);

create table if not exists message_attachments (
  id text primary key,
  message_id text not null references messages(id) on delete cascade,
  url text not null,
  filename text not null,
  content_type text,
  size_bytes integer not null default 0,
  height integer,
  width integer,
  created_at timestamptz not null default now()
);

create index if not exists message_attachments_message_idx
  on message_attachments (message_id);

create table if not exists message_embeddings (
  message_id text primary key references messages(id) on delete cascade,
  embedding vector(1536) not null,
  model text not null,
  embedded_text text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create or replace function count_message_phrase(
  filter_guild_id text default null,
  filter_dm_user_id text default null,
  filter_channel_id text default null,
  filter_author_user_id text default null,
  phrase text default ''
)
returns bigint
language sql
stable
as $$
  select count(*)::bigint
  from messages m
  where phrase <> ''
    and (filter_guild_id is null or m.guild_id = filter_guild_id)
    and (filter_dm_user_id is null or m.dm_user_id = filter_dm_user_id)
    and (filter_channel_id is null or m.channel_id = filter_channel_id)
    and (filter_author_user_id is null or m.author_user_id = filter_author_user_id)
    and m.content ilike '%' || phrase || '%';
$$;

create or replace function match_message_embeddings(
  query_embedding vector(1536),
  match_count integer default 8,
  filter_guild_id text default null,
  filter_dm_user_id text default null,
  filter_channel_id text default null
)
returns table (
  id text,
  guild_id text,
  channel_id text,
  thread_id text,
  dm_user_id text,
  author_user_id text,
  author_username text,
  author_is_bot boolean,
  content text,
  created_at timestamptz,
  similarity double precision
)
language sql
stable
as $$
  select
    m.id,
    m.guild_id,
    m.channel_id,
    m.thread_id,
    m.dm_user_id,
    m.author_user_id,
    m.author_username,
    m.author_is_bot,
    m.content,
    m.created_at,
    1 - (e.embedding <=> query_embedding) as similarity
  from message_embeddings e
  join messages m on m.id = e.message_id
  where (filter_guild_id is null or m.guild_id = filter_guild_id)
    and (filter_dm_user_id is null or m.dm_user_id = filter_dm_user_id)
    and (filter_channel_id is null or m.channel_id = filter_channel_id)
  order by e.embedding <=> query_embedding
  limit greatest(match_count, 1);
$$;
