import { stdin as input } from 'node:process';

import { loadEnv } from '../src/config/env.js';
import { SupabaseSensitiveDataStore } from '../src/adapters/supabaseSensitiveData.js';
import { Logger } from '../src/lib/logger.js';
import { createSupabaseAdminClient } from '../src/lib/supabase.js';
import { SensitiveDataService } from '../src/services/sensitiveDataService.js';

async function main(): Promise<void> {
  const env = loadEnv();
  const logger = new Logger('error');
  const service = new SensitiveDataService(
    env,
    new SupabaseSensitiveDataStore(createSupabaseAdminClient(env)),
    logger
  );

  const [command, ...args] = process.argv.slice(2);
  const options = parseArgs(args);

  if (command === 'put') {
    const guildId = requireOption(options, 'guild');
    const ownerUserId = requireOption(options, 'owner');
    const label = requireOption(options, 'label');
    const createdByUserId = options.grantedBy ?? options.createdBy ?? ownerUserId;
    const value = (await readStdin()).trimEnd();

    if (value.length === 0) {
      throw new Error('Provide the sensitive value through stdin.');
    }

    await service.putRecord({
      createdByUserId,
      description: options.description ?? null,
      guildId,
      label,
      ownerUserId,
      value
    });

    console.log(`Stored sensitive record "${label}" for ${ownerUserId}.`);
    return;
  }

  if (command === 'delete') {
    const guildId = requireOption(options, 'guild');
    const ownerUserId = requireOption(options, 'owner');
    const label = requireOption(options, 'label');
    const deleted = await service.deleteRecord(guildId, ownerUserId, label);
    console.log(
      deleted
        ? `Deleted sensitive record "${label}" for ${ownerUserId}.`
        : `No sensitive record "${label}" existed for ${ownerUserId}.`
    );
    return;
  }

  if (command === 'list') {
    const guildId = requireOption(options, 'guild');
    const ownerUserId = requireOption(options, 'owner');
    const summaries = await service.listRecordSummaries(guildId, ownerUserId);

    if (summaries.length === 0) {
      console.log(`No sensitive records stored for ${ownerUserId}.`);
      return;
    }

    console.log(`Sensitive records for ${ownerUserId}:`);
    for (const summary of summaries) {
      console.log(summary.description ? `- ${summary.label}: ${summary.description}` : `- ${summary.label}`);
    }
    return;
  }

  throw new Error('Usage: tsx scripts/manage-sensitive-data.ts <put|delete|list> --guild GUILD_ID --owner USER_ID [--label LABEL] [--description TEXT]');
}

function parseArgs(args: string[]): Record<string, string> {
  const options: Record<string, string> = {};

  for (let index = 0; index < args.length; index += 1) {
    const current = args[index];
    if (!current?.startsWith('--')) {
      continue;
    }

    const key = current.replace(/^--/, '');
    const value = args[index + 1];
    if (!value || value.startsWith('--')) {
      throw new Error(`Missing value for --${key}`);
    }

    options[key] = value;
    index += 1;
  }

  return options;
}

function requireOption(options: Record<string, string>, key: string): string {
  const value = options[key];
  if (!value) {
    throw new Error(`Missing required option --${key}`);
  }

  return value;
}

async function readStdin(): Promise<string> {
  const chunks: Buffer[] = [];

  for await (const chunk of input) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }

  return Buffer.concat(chunks).toString('utf8');
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
