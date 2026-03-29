-- Reserved migration number kept to preserve the checked-in history.
--
-- This file used to drop control-plane tables from an abandoned direction.
-- The current bot runtime depends on `channel_ingestion_policies`, so this
-- migration must remain a no-op for fresh CLI-managed environments.
do $$
begin
  null;
end
$$;
