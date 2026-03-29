import type OpenAI from 'openai';

import type { EmbeddingClient, ResponseClient } from '../ports/ai.js';

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
