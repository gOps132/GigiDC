import type { SupabaseClient } from '@supabase/supabase-js';

export interface AuditLogInput {
  guildId: string | null;
  actorUserId: string | null;
  action: string;
  targetType: string;
  targetId?: string | null;
  metadata?: Record<string, unknown>;
}

export class AuditLogService {
  constructor(private readonly supabase: SupabaseClient) {}

  async record(input: AuditLogInput): Promise<void> {
    const { error } = await this.supabase.from('audit_logs').insert({
      guild_id: input.guildId,
      actor_user_id: input.actorUserId,
      action: input.action,
      target_type: input.targetType,
      target_id: input.targetId ?? null,
      metadata: input.metadata ?? {}
    });

    if (error) {
      throw new Error(`Failed to write audit log: ${error.message}`);
    }
  }
}
