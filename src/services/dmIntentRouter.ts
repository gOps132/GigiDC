export type DmIntentRoute =
  | {
      kind: 'cancel_pending_action';
    }
  | {
      kind: 'confirm_pending_action';
    }
  | {
      kind: 'direct_reply';
      reply: string;
    }
  | {
      kind: 'retrieval';
    }
  | {
      kind: 'tool_request';
    };

export class DmIntentRouter {
  route(query: string): DmIntentRoute {
    const directReply = getDeterministicDmReply(query);
    if (directReply) {
      return {
        kind: 'direct_reply',
        reply: directReply
      };
    }

    if (looksLikeCancelRequest(query)) {
      return {
        kind: 'cancel_pending_action'
      };
    }

    if (looksLikeConfirmationRequest(query)) {
      return {
        kind: 'confirm_pending_action'
      };
    }

    if (looksLikeToolRequest(query)) {
      return {
        kind: 'tool_request'
      };
    }

    return {
      kind: 'retrieval'
    };
  }
}

export function getDeterministicDmReply(query: string): string | null {
  const normalized = query.trim().toLowerCase();

  if (
    /\b(what tools can you call|what tools do you have|what can you do|what capabilities do you have|what are your capabilities)\b/i.test(
      normalized
    )
  ) {
    return [
      'In this bot runtime I can:',
      '- chat in DM and when you mention me in a server channel',
      '- answer from your DM history',
      '- answer from the current channel when you mention me there',
      '- answer from permitted primary-server history when you have access',
      '- count exact phrases from stored history',
      '- recall participant-visible relays and tasks',
      '- manage ingestion status and assignment workflows in DM when your guild permissions allow it',
      '- list or manage direct user permissions in DM when your guild permissions allow it',
      '- inspect token usage and estimated USD cost in DM when your guild permissions allow it',
      '- list or retrieve your own stored sensitive records in DM',
      '- create, list, and complete tasks',
      '- request and confirm permission-gated Gigi-mediated DMs',
      '',
      'I cannot browse the web, run code, provide a sandbox, generate images, or use arbitrary external tools here.'
    ].join('\n');
  }

  if (
    /\b(code execution|execute code|run code|sandbox|shell access|terminal access|browser|browse the web|web search|search the web|image generation|generate images|translation|translate)\b/i.test(
      normalized
    )
  ) {
    return 'No. In this bot runtime I cannot run code, provide a sandbox, browse the web, generate images, or use arbitrary external tools. The internal tools here are bounded to task management, permission-gated DM relays, permission-gated ingestion and assignment admin actions, permission management, usage reporting, and DM-only sensitive-data retrieval.';
  }

  if (/^(how.?s ingestion going|how is ingestion going|what.?s ingestion status)[.!?]*$/i.test(normalized)) {
    return 'I can manage ingestion in DM when you have the right guild permission, but you need to name the target channel. Ask something like "show ingestion status for general" or "enable ingestion for #shipping".';
  }

  return null;
}

export function looksLikeToolRequest(query: string): boolean {
  return /\b(task|tasks|todo|to-do|remind|reminder|follow up|follow-up|complete|completed|done|finish|mark .* done|relay|send .* dm|can you dm|dm .* to|dm @|message @|message .* via dm|assign|assignment|assignments|publish assignment|ingestion|channel ingestion|permission|permissions|capability|capabilities|grant .* permission|revoke .* permission|usage|token usage|cost summary|how much.*cost|estimated cost)\b/i.test(
    query
  );
}

function looksLikeConfirmationRequest(query: string): boolean {
  return /^(confirm|confirm it|yes|yes send it|go ahead|send it|approve|approved|do it)[.!?]*$/i.test(
    query.trim()
  );
}

function looksLikeCancelRequest(query: string): boolean {
  return /^(cancel|cancel it|never mind|nevermind|stop|don.?t send it)[.!?]*$/i.test(
    query.trim()
  );
}
