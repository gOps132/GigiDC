alter table llm_guild_policies
  add column if not exists tool_routing_mode text not null default 'off';

alter table llm_guild_policies
  drop constraint if exists llm_guild_policies_tool_routing_mode_check;

alter table llm_guild_policies
  add constraint llm_guild_policies_tool_routing_mode_check
  check (tool_routing_mode in ('off', 'dry-run', 'enabled'));
