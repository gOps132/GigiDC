create table if not exists model_usage_events (
  id uuid primary key default gen_random_uuid(),
  guild_id text,
  channel_id text,
  requester_user_id text,
  message_id text,
  provider text not null,
  operation text not null,
  surface text not null,
  model text not null,
  input_tokens integer,
  output_tokens integer,
  total_tokens integer,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  constraint model_usage_events_input_tokens_nonnegative check (input_tokens is null or input_tokens >= 0),
  constraint model_usage_events_output_tokens_nonnegative check (output_tokens is null or output_tokens >= 0),
  constraint model_usage_events_total_tokens_nonnegative check (total_tokens is null or total_tokens >= 0)
);

create index if not exists model_usage_events_created_at_idx
  on model_usage_events (created_at desc);

create index if not exists model_usage_events_requester_created_at_idx
  on model_usage_events (requester_user_id, created_at desc);

create index if not exists model_usage_events_operation_created_at_idx
  on model_usage_events (operation, created_at desc);

create or replace function estimated_openai_input_cost_per_million(model_name text)
returns numeric
language sql
immutable
as $$
  select case
    when lower(model_name) like 'gpt-4.1-mini%' then 0.40
    when lower(model_name) like 'gpt-4.1-nano%' then 0.10
    when lower(model_name) like 'gpt-4.1%' then 2.00
    when lower(model_name) like 'gpt-4o-mini%' then 0.15
    when lower(model_name) like 'text-embedding-3-small%' then 0.02
    when lower(model_name) like 'text-embedding-3-large%' then 0.13
    when lower(model_name) like 'text-embedding-ada-002%' then 0.10
    else null
  end;
$$;

create or replace function estimated_openai_output_cost_per_million(model_name text)
returns numeric
language sql
immutable
as $$
  select case
    when lower(model_name) like 'gpt-4.1-mini%' then 1.60
    when lower(model_name) like 'gpt-4.1-nano%' then 0.40
    when lower(model_name) like 'gpt-4.1%' then 8.00
    when lower(model_name) like 'gpt-4o-mini%' then 0.60
    when lower(model_name) like 'text-embedding-3-small%' then 0.00
    when lower(model_name) like 'text-embedding-3-large%' then 0.00
    when lower(model_name) like 'text-embedding-ada-002%' then 0.00
    else null
  end;
$$;

create or replace function estimated_model_usage_cost_usd(
  provider_name text,
  model_name text,
  input_token_count integer,
  output_token_count integer
)
returns numeric
language sql
immutable
as $$
  select case
    when lower(provider_name) <> 'openai' then null
    else
      (
        coalesce(input_token_count, 0)::numeric / 1000000
        * coalesce(estimated_openai_input_cost_per_million(model_name), 0)
      ) +
      (
        coalesce(output_token_count, 0)::numeric / 1000000
        * coalesce(estimated_openai_output_cost_per_million(model_name), 0)
      )
  end;
$$;

create or replace view model_usage_event_estimates as
select
  id,
  created_at,
  guild_id,
  channel_id,
  requester_user_id,
  message_id,
  provider,
  operation,
  surface,
  model,
  input_tokens,
  output_tokens,
  total_tokens,
  metadata,
  estimated_openai_input_cost_per_million(model) as estimated_input_usd_per_million_tokens,
  estimated_openai_output_cost_per_million(model) as estimated_output_usd_per_million_tokens,
  estimated_model_usage_cost_usd(provider, model, input_tokens, output_tokens) as estimated_cost_usd
from model_usage_events;

create or replace view model_usage_daily_summary as
select
  date_trunc('day', created_at) as usage_day,
  provider,
  model,
  operation,
  surface,
  count(*) as event_count,
  coalesce(sum(input_tokens), 0) as input_tokens,
  coalesce(sum(output_tokens), 0) as output_tokens,
  coalesce(sum(total_tokens), 0) as total_tokens,
  round(coalesce(sum(estimated_cost_usd), 0)::numeric, 8) as estimated_cost_usd
from model_usage_event_estimates
group by 1, 2, 3, 4, 5;

create or replace view model_usage_requester_daily_summary as
select
  date_trunc('day', created_at) as usage_day,
  guild_id,
  requester_user_id,
  provider,
  operation,
  surface,
  count(*) as event_count,
  coalesce(sum(input_tokens), 0) as input_tokens,
  coalesce(sum(output_tokens), 0) as output_tokens,
  coalesce(sum(total_tokens), 0) as total_tokens,
  round(coalesce(sum(estimated_cost_usd), 0)::numeric, 8) as estimated_cost_usd
from model_usage_event_estimates
group by 1, 2, 3, 4, 5, 6;
