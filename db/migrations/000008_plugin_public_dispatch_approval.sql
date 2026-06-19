alter table plugin_versions
  add column if not exists public_dispatch_allowed boolean not null default false;
