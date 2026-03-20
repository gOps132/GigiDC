import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http';

import type { Client } from 'discord.js';

import type { BotContext } from '../discord/types.js';
import type { ClawbotClient } from '../services/clawbotClient.js';
import { ClawbotResultPostingService } from '../services/clawbotResultPostingService.js';
import { ClawbotWebhookService } from '../services/clawbotWebhookService.js';

export function startWebhookServer(
  context: BotContext,
  client: Client,
  clawbotClient: ClawbotClient
): Server {
  const resultPoster = new ClawbotResultPostingService(client, context.logger);
  const webhookService = new ClawbotWebhookService(context, client, resultPoster);

  const server = createServer(async (request, response) => {
    try {
      await handleRequest(request, response, context, clawbotClient, webhookService);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown webhook server error';
      context.logger.error('Webhook server request failed', { error: message });
      writeJson(response, 500, { error: 'Internal server error' });
    }
  });

  const port = context.env.PORT;
  server.listen(port, () => {
    context.logger.info('Webhook server listening', { port });
  });

  return server;
}

async function handleRequest(
  request: IncomingMessage,
  response: ServerResponse,
  context: BotContext,
  clawbotClient: ClawbotClient,
  webhookService: ClawbotWebhookService
): Promise<void> {
  const requestUrl = new URL(request.url ?? '/', 'http://localhost');

  if (request.method === 'GET' && requestUrl.pathname === '/healthz') {
    writeJson(response, 200, { ok: true });
    return;
  }

  if (request.method === 'POST' && requestUrl.pathname === '/webhooks/clawbot') {
    const authHeader = request.headers.authorization;
    const secretHeader = request.headers['x-clawbot-webhook-secret'];
    const expectedSecret = clawbotClient.webhookSecret();

    const providedSecret = authHeader?.startsWith('Bearer ')
      ? authHeader.slice('Bearer '.length)
      : typeof secretHeader === 'string'
        ? secretHeader
        : null;

    if (providedSecret !== expectedSecret) {
      writeJson(response, 401, { error: 'Unauthorized' });
      return;
    }

    const body = await readJsonBody(request);
    const payload = webhookService.parsePayload(body);
    await webhookService.handleCompletion(payload);
    writeJson(response, 200, { ok: true });
    return;
  }

  writeJson(response, 404, { error: 'Not found' });
}

async function readJsonBody(request: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];

  for await (const chunk of request) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }

  const raw = Buffer.concat(chunks).toString('utf8');
  return raw.length > 0 ? JSON.parse(raw) : {};
}

function writeJson(response: ServerResponse, statusCode: number, payload: object): void {
  const body = JSON.stringify(payload);
  response.writeHead(statusCode, {
    'content-type': 'application/json',
    'content-length': Buffer.byteLength(body)
  });
  response.end(body);
}
