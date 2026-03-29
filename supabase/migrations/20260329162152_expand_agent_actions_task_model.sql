alter table agent_actions
  add column if not exists action_scope text not null default 'action',
  add column if not exists due_at timestamptz;

alter table agent_actions
  drop constraint if exists agent_actions_status_check;

alter table agent_actions
  add constraint agent_actions_status_check check (
    status in ('requested', 'in_progress', 'completed', 'failed', 'cancelled')
  );

alter table agent_actions
  add constraint agent_actions_scope_check check (
    action_scope in ('action', 'task')
  );

create index if not exists agent_actions_scope_status_created_at_idx
  on agent_actions (action_scope, status, created_at desc);

create index if not exists agent_actions_open_task_assignee_due_idx
  on agent_actions (recipient_user_id, due_at asc, created_at desc)
  where action_scope = 'task'
    and status in ('requested', 'in_progress');
