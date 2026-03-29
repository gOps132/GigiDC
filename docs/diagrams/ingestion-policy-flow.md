# Ingestion Policy Flow

This diagram captures the new admin path for channel-ingestion control: permission check, policy write, cache refresh, and audit logging.

```mermaid
flowchart TD
  Admin["Discord Admin User"]
  Discord["Discord Slash Command"]
  Command["/ingestion enable|disable|status"]
  RolePolicy["RolePolicyService"]
  Ingestion["ChannelIngestionPolicyService"]
  Audit["AuditLogService"]
  PolicyStore["channel_ingestion_policies"]
  AuditStore["audit_logs"]

  Admin --> Discord
  Discord --> Command
  Command --> RolePolicy
  RolePolicy -->|"ingestion_admin or Discord Administrator"| Command
  Command -->|"status / read current state"| Ingestion
  Command -->|"enable / disable"| Ingestion
  Ingestion --> PolicyStore
  Command -->|"permission denied or policy change"| Audit
  Audit --> AuditStore
```

## Reading Guide

- `ingestion_admin` is the explicit capability for changing retention behavior on guild channels.
- `ChannelIngestionPolicyService` now handles both reads and writes, and refreshes its cache after policy updates.
- Policy changes and denied admin attempts are written to `audit_logs` so ingestion-control decisions leave a trail.
- This flow is the current operability seam for guild-history retention.
