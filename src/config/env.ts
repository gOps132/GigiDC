import { config as loadDotEnv } from 'dotenv';
import { z } from 'zod';

loadDotEnv();

const envSchema = z.object({
  NODE_ENV: z.enum(['development', 'test', 'production']).default('development'),
  LOG_LEVEL: z.enum(['debug', 'info', 'warn', 'error']).default('info'),
  PORT: z.coerce.number().int().min(1).max(65535).default(8080),
  DISCORD_TOKEN: z.string().min(1, 'DISCORD_TOKEN is required'),
  DISCORD_CLIENT_ID: z.string().min(1, 'DISCORD_CLIENT_ID is required'),
  DISCORD_GUILD_ID: z.string().min(1).optional(),
  SUPABASE_URL: z.string().url('SUPABASE_URL must be a valid URL'),
  SUPABASE_SERVICE_ROLE_KEY: z.string().min(1, 'SUPABASE_SERVICE_ROLE_KEY is required'),
  BOT_PUBLIC_BASE_URL: z.string().url('BOT_PUBLIC_BASE_URL must be a valid URL'),
  CLAWBOT_BASE_URL: z.string().url('CLAWBOT_BASE_URL must be a valid URL'),
  CLAWBOT_API_KEY: z.string().min(1, 'CLAWBOT_API_KEY is required'),
  CLAWBOT_WEBHOOK_SECRET: z.string().min(1).optional(),
  CLAWBOT_JOB_PATH: z.string().min(1).default('/api/v1/jobs'),
  CLAWBOT_INGEST_PATH: z.string().min(1).default('/api/v1/ingest/discord-message')
});

export type Env = z.infer<typeof envSchema>;

export function loadEnv(): Env {
  const parsed = envSchema.safeParse(process.env);

  if (parsed.success) {
    return parsed.data;
  }

  const message = parsed.error.issues
    .map((issue) => `${issue.path.join('.')}: ${issue.message}`)
    .join('\n');

  throw new Error(`Invalid environment configuration:\n${message}`);
}
