import type { AuditLogStore } from '../ports/controlPlane.js';

export interface AuditLogInput {
  guildId: string | null;
  actorUserId: string | null;
  action: string;
  targetType: string;
  targetId?: string | null;
  metadata?: Record<string, unknown>;
}

export class AuditLogService {
  constructor(private readonly store: AuditLogStore) {}

  async record(input: AuditLogInput): Promise<void> {
    await this.store.record(input);
  }
}
