import type { SupabaseClient } from '@supabase/supabase-js';

export type ClawbotJobStatus =
  | 'queued'
  | 'submitted'
  | 'running'
  | 'completed'
  | 'failed'
  | 'cancelled';

export interface CreateClawbotJobInput {
  guildId: string;
  channelId: string;
  threadId: string | null;
  requesterUserId: string;
  commandName: string;
  taskType: string;
  requestPayload: Record<string, unknown>;
}

export interface UpdateClawbotSubmissionInput {
  localJobId: string;
  clawbotJobId: string;
  status: Extract<ClawbotJobStatus, 'submitted' | 'running'>;
}

export interface CompleteClawbotJobInput {
  localJobId: string;
  clawbotJobId: string | null;
  status: Extract<ClawbotJobStatus, 'completed' | 'failed' | 'cancelled'>;
  resultSummary: string | null;
  artifactLinks: string[];
  errorMessage: string | null;
}

export interface ClawbotJobRecord {
  id: string;
  guild_id: string;
  channel_id: string;
  thread_id: string | null;
  requester_user_id: string;
  command_name: string;
  task_type: string;
  status: ClawbotJobStatus;
  clawbot_job_id: string | null;
  result_summary: string | null;
  artifact_links: string[];
  error_message: string | null;
  result_posted_message_id: string | null;
  created_at: string;
  updated_at: string;
  completed_at: string | null;
}

export class ClawbotJobService {
  constructor(private readonly supabase: SupabaseClient) {}

  async createJob(input: CreateClawbotJobInput): Promise<ClawbotJobRecord> {
    const { data, error } = await this.supabase
      .from('clawbot_jobs')
      .insert({
        guild_id: input.guildId,
        channel_id: input.channelId,
        thread_id: input.threadId,
        requester_user_id: input.requesterUserId,
        command_name: input.commandName,
        task_type: input.taskType,
        request_payload: input.requestPayload
      })
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to create Clawbot job: ${error?.message ?? 'Unknown error'}`);
    }

    return mapJobRecord(data);
  }

  async markSubmitted(input: UpdateClawbotSubmissionInput): Promise<ClawbotJobRecord> {
    const { data, error } = await this.supabase
      .from('clawbot_jobs')
      .update({
        clawbot_job_id: input.clawbotJobId,
        status: input.status,
        updated_at: new Date().toISOString()
      })
      .eq('id', input.localJobId)
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to update Clawbot job submission: ${error?.message ?? 'Unknown error'}`);
    }

    return mapJobRecord(data);
  }

  async markComplete(input: CompleteClawbotJobInput): Promise<ClawbotJobRecord> {
    const { data, error } = await this.supabase
      .from('clawbot_jobs')
      .update({
        clawbot_job_id: input.clawbotJobId,
        status: input.status,
        result_summary: input.resultSummary,
        artifact_links: input.artifactLinks,
        error_message: input.errorMessage,
        updated_at: new Date().toISOString(),
        completed_at: new Date().toISOString()
      })
      .eq('id', input.localJobId)
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to complete Clawbot job: ${error?.message ?? 'Unknown error'}`);
    }

    return mapJobRecord(data);
  }

  async markResultPosted(localJobId: string, messageId: string): Promise<void> {
    const { error } = await this.supabase
      .from('clawbot_jobs')
      .update({
        result_posted_message_id: messageId,
        updated_at: new Date().toISOString()
      })
      .eq('id', localJobId);

    if (error) {
      throw new Error(`Failed to mark result as posted: ${error.message}`);
    }
  }

  async findByLocalId(localJobId: string): Promise<ClawbotJobRecord | null> {
    const { data, error } = await this.supabase
      .from('clawbot_jobs')
      .select('*')
      .eq('id', localJobId)
      .maybeSingle();

    if (error) {
      throw new Error(`Failed to load Clawbot job: ${error.message}`);
    }

    return data ? mapJobRecord(data) : null;
  }
}

function mapJobRecord(data: Record<string, unknown>): ClawbotJobRecord {
  return {
    id: String(data.id),
    guild_id: String(data.guild_id),
    channel_id: String(data.channel_id),
    thread_id: data.thread_id ? String(data.thread_id) : null,
    requester_user_id: String(data.requester_user_id),
    command_name: String(data.command_name),
    task_type: String(data.task_type),
    status: data.status as ClawbotJobStatus,
    clawbot_job_id: data.clawbot_job_id ? String(data.clawbot_job_id) : null,
    result_summary: data.result_summary ? String(data.result_summary) : null,
    artifact_links: Array.isArray(data.artifact_links)
      ? data.artifact_links.map((item) => String(item))
      : [],
    error_message: data.error_message ? String(data.error_message) : null,
    result_posted_message_id: data.result_posted_message_id
      ? String(data.result_posted_message_id)
      : null,
    created_at: String(data.created_at),
    updated_at: String(data.updated_at),
    completed_at: data.completed_at ? String(data.completed_at) : null
  };
}
