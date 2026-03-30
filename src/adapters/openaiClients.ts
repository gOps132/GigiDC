import type OpenAI from 'openai';
import { zodTextFormat } from 'openai/helpers/zod';
import { z } from 'zod';

import type { EmbeddingClient, ResponseClient, ToolPlan, ToolPlanningClient } from '../ports/ai.js';

const toolPlanSchema = z.object({
  toolCalls: z
    .array(
      z.discriminatedUnion('name', [
        z.object({
          name: z.literal('create_follow_up_task'),
          title: z.string().min(1).max(120),
          details: z.string().min(1).max(2000),
          assigneeReference: z.string().trim().min(1).max(120).nullable(),
          dueAt: z.string().trim().min(1).max(80).nullable()
        }),
        z.object({
          name: z.literal('list_open_tasks'),
          userReference: z.string().trim().min(1).max(120).nullable()
        }),
        z.object({
          name: z.literal('get_ingestion_status'),
          channelReference: z.string().trim().min(1).max(120).nullable()
        }),
        z.object({
          name: z.literal('set_ingestion_policy'),
          channelReference: z.string().trim().min(1).max(120).nullable(),
          enabled: z.boolean()
        }),
        z.object({
          name: z.literal('create_assignment'),
          title: z.string().trim().min(1).max(160),
          description: z.string().trim().min(1).max(3000),
          dueAt: z.string().trim().min(1).max(80).nullable(),
          channelReference: z.string().trim().min(1).max(120).nullable(),
          affectedRoleReferences: z.array(z.string().trim().min(1).max(120)).max(8).default([])
        }),
        z.object({
          name: z.literal('list_assignments')
        }),
        z.object({
          name: z.literal('publish_assignment'),
          assignmentReference: z.string().trim().min(1).max(160),
          channelReference: z.string().trim().min(1).max(120).nullable()
        }),
        z.object({
          name: z.literal('grant_permission'),
          userReference: z.string().trim().min(1).max(120),
          capability: z.string().trim().min(1).max(120)
        }),
        z.object({
          name: z.literal('revoke_permission'),
          userReference: z.string().trim().min(1).max(120),
          capability: z.string().trim().min(1).max(120)
        }),
        z.object({
          name: z.literal('list_permissions'),
          userReference: z.string().trim().min(1).max(120).nullable()
        }),
        z.object({
          name: z.literal('complete_task'),
          taskReference: z.string().trim().min(1).max(120),
          result: z.string().trim().min(1).max(500).nullable()
        }),
        z.object({
          name: z.literal('send_dm_relay'),
          recipientReference: z.string().trim().min(1).max(120),
          message: z.string().min(1).max(2000),
          context: z.string().trim().min(1).max(500).nullable()
        })
      ])
    )
    .max(3)
    .default([])
});

export class OpenAIEmbeddingClient implements EmbeddingClient {
  constructor(private readonly openai: OpenAI) {}

  async createEmbedding(model: string, input: string): Promise<number[]> {
    const response = await this.openai.embeddings.create({
      model,
      input
    });

    const vector = response.data[0]?.embedding;
    if (!vector) {
      throw new Error('OpenAI embedding response was empty');
    }

    return vector;
  }
}

export class OpenAIResponseClient implements ResponseClient {
  constructor(private readonly openai: OpenAI) {}

  async createTextResponse(input: {
    instructions: string;
    model: string;
    text: string;
  }): Promise<string> {
    const response = await this.openai.responses.create({
      model: input.model,
      instructions: input.instructions,
      input: [
        {
          role: 'user',
          content: [
            {
              type: 'input_text',
              text: input.text
            }
          ]
        }
      ]
    });

    return response.output_text.trim() || 'I could not produce a useful answer for that yet.';
  }
}

export class OpenAIToolPlanningClient implements ToolPlanningClient {
  constructor(private readonly openai: OpenAI) {}

  async planDmTools(input: {
    instructions: string;
    model: string;
    text: string;
  }): Promise<ToolPlan> {
    const response = await this.openai.responses.parse({
      model: input.model,
      instructions: input.instructions,
      input: input.text,
      max_output_tokens: 600,
      text: {
        format: zodTextFormat(toolPlanSchema, 'gigi_dm_tool_plan'),
        verbosity: 'low'
      }
    });

    return response.output_parsed ?? {
      toolCalls: []
    };
  }
}
