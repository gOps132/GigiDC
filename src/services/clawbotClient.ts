import type { Env } from '../config/env.js';
import { fetchJson } from '../lib/http.js';

export interface ClawbotDispatchPayload {
  localJobId: string;
  commandName: string;
  taskType: string;
  guildId: string;
  channelId: string;
  threadId: string | null;
  requesterUserId: string;
  input: Record<string, unknown>;
  callbackUrl: string;
}

export interface ClawbotDispatchResponse {
  jobId: string;
  status: 'submitted' | 'running';
}

export interface ClawbotIngestionPayload {
  guildId: string;
  channelId: string;
  threadId: string | null;
  messageId: string;
  authorId: string;
  authorUsername: string;
  content: string;
  createdAt: string;
  attachmentMetadata: Array<{
    id: string;
    name: string;
    contentType: string | null;
    size: number;
    url: string;
  }>;
}

export class ClawbotClient {
  constructor(private readonly env: Env) {}

  async dispatchJob(payload: ClawbotDispatchPayload): Promise<ClawbotDispatchResponse> {
    const url = new URL(this.env.CLAWBOT_JOB_PATH, ensureTrailingSlash(this.env.CLAWBOT_BASE_URL));
    return fetchJson<ClawbotDispatchResponse>(url, {
      method: 'POST',
      headers: this.buildHeaders(),
      body: JSON.stringify(payload)
    });
  }

  async ingestDiscordMessage(payload: ClawbotIngestionPayload): Promise<void> {
    const url = new URL(this.env.CLAWBOT_INGEST_PATH, ensureTrailingSlash(this.env.CLAWBOT_BASE_URL));
    await fetchJson(url, {
      method: 'POST',
      headers: this.buildHeaders(),
      body: JSON.stringify(payload)
    });
  }

  webhookSecret(): string {
    return this.env.CLAWBOT_WEBHOOK_SECRET ?? this.env.CLAWBOT_API_KEY;
  }

  private buildHeaders(): HeadersInit {
    return {
      authorization: `Bearer ${this.env.CLAWBOT_API_KEY}`,
      'content-type': 'application/json'
    };
  }
}

function ensureTrailingSlash(value: string): string {
  return value.endsWith('/') ? value : `${value}/`;
}
