import type { AssignmentStore } from '../ports/controlPlane.js';

export interface AssignmentRecord {
  id: string;
  guild_id: string;
  title: string;
  description: string;
  due_at: string | null;
  announcement_channel_id: string | null;
  mentioned_role_ids: string[];
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
  mentionedRoleIds: string[];
  createdByUserId: string;
}

export class AssignmentService {
  constructor(private readonly store: AssignmentStore) {}

  async createAssignment(input: CreateAssignmentInput): Promise<AssignmentRecord> {
    return this.store.createAssignment(input);
  }

  async getAssignmentById(guildId: string, assignmentId: string): Promise<AssignmentRecord | null> {
    return this.store.getAssignmentById(guildId, assignmentId);
  }

  async listAssignments(guildId: string): Promise<AssignmentRecord[]> {
    return this.store.listAssignments(guildId);
  }

  async markPublished(
    guildId: string,
    assignmentId: string,
    messageId: string,
    channelId: string
  ): Promise<AssignmentRecord> {
    return this.store.markPublished(guildId, assignmentId, messageId, channelId);
  }
}
