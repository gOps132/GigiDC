import type { SupabaseClient } from '@supabase/supabase-js';

export interface AssignmentRecord {
  id: string;
  guild_id: string;
  title: string;
  description: string;
  due_at: string | null;
  announcement_channel_id: string | null;
  created_by_user_id: string;
  published_message_id: string | null;
  status: 'draft' | 'published';
  created_at: string;
  updated_at: string;
}

export interface CreateAssignmentInput {
  guildId: string;
  title: string;
  description: string;
  dueAt: string | null;
  announcementChannelId: string | null;
  createdByUserId: string;
}

export class AssignmentService {
  constructor(private readonly supabase: SupabaseClient) {}

  async createAssignment(input: CreateAssignmentInput): Promise<AssignmentRecord> {
    const { data, error } = await this.supabase
      .from('assignments')
      .insert({
        guild_id: input.guildId,
        title: input.title,
        description: input.description,
        due_at: input.dueAt,
        announcement_channel_id: input.announcementChannelId,
        created_by_user_id: input.createdByUserId
      })
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to create assignment: ${error?.message ?? 'Unknown error'}`);
    }

    return data as AssignmentRecord;
  }

  async getAssignmentById(guildId: string, assignmentId: string): Promise<AssignmentRecord | null> {
    const { data, error } = await this.supabase
      .from('assignments')
      .select('*')
      .eq('guild_id', guildId)
      .eq('id', assignmentId)
      .maybeSingle();

    if (error) {
      throw new Error(`Failed to load assignment: ${error.message}`);
    }

    return (data as AssignmentRecord | null) ?? null;
  }

  async listAssignments(guildId: string): Promise<AssignmentRecord[]> {
    const { data, error } = await this.supabase
      .from('assignments')
      .select('*')
      .eq('guild_id', guildId)
      .order('created_at', { ascending: false })
      .limit(10);

    if (error) {
      throw new Error(`Failed to list assignments: ${error.message}`);
    }

    return (data ?? []) as AssignmentRecord[];
  }

  async markPublished(
    guildId: string,
    assignmentId: string,
    messageId: string,
    channelId: string
  ): Promise<AssignmentRecord> {
    const { data, error } = await this.supabase
      .from('assignments')
      .update({
        status: 'published',
        published_message_id: messageId,
        announcement_channel_id: channelId,
        updated_at: new Date().toISOString()
      })
      .eq('guild_id', guildId)
      .eq('id', assignmentId)
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to publish assignment: ${error?.message ?? 'Unknown error'}`);
    }

    return data as AssignmentRecord;
  }
}
