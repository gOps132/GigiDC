drop view if exists model_usage_daily_summary;

create view model_usage_daily_summary as
select
  date_trunc('day', created_at) as usage_day,
  guild_id,
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
group by 1, 2, 3, 4, 5, 6;
