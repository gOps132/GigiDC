import OpenAI from 'openai';

import type { Env } from '../config/env.js';

export function createOpenAIClient(env: Env): OpenAI {
  return new OpenAI({
    apiKey: env.OPENAI_API_KEY
  });
}
