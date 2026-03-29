import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http';

import type { BotContext } from '../discord/types.js';

export function startWebhookServer(context: BotContext): Server {
  const server = createServer(async (request, response) => {
    try {
      await handleRequest(request, response, context);
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
  context: BotContext
): Promise<void> {
  const requestUrl = new URL(request.url ?? '/', 'http://localhost');
  const runtimeSnapshot = context.runtime.getSnapshot(context.services.messageIndexing.getStatus());

  if (request.method === 'GET' && requestUrl.pathname === '/healthz') {
    writeJson(response, 200, {
      ok: true,
      runtime: runtimeSnapshot
    });
    return;
  }

  if (request.method === 'GET' && requestUrl.pathname === '/readyz') {
    writeJson(response, runtimeSnapshot.ready ? 200 : 503, runtimeSnapshot);
    return;
  }

  writeJson(response, 404, { error: 'Not found' });
}

function writeJson(response: ServerResponse, statusCode: number, payload: object): void {
  const body = JSON.stringify(payload);
  response.writeHead(statusCode, {
    'content-type': 'application/json',
    'content-length': Buffer.byteLength(body)
  });
  response.end(body);
}
