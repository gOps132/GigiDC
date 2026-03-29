import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import type { ResponseClient } from '../ports/ai.js';
import type { AgentActionRecord, AgentActionService } from './agentActionService.js';
import type { HistoryMessageRecord, HistoryScope, MessageHistoryService } from './messageHistoryService.js';

export interface RetrievalAnswer {
  answer: string;
  source: 'action' | 'exact' | 'semantic' | 'direct';
}

export class RetrievalService {
  constructor(
    private readonly env: Env,
    private readonly responses: ResponseClient,
    private readonly messageHistory: MessageHistoryService,
    private readonly agentActions: AgentActionService,
    private readonly logger: Logger
  ) {}

  async answerQuestion(
    query: string,
    scope: HistoryScope,
    requesterUserId: string,
    botUserId: string
  ): Promise<RetrievalAnswer> {
    const capabilityAnswer = parseCapabilityIntent(query);
    if (capabilityAnswer) {
      return {
        answer: capabilityAnswer,
        source: 'direct'
      };
    }

    const phraseCountIntent = parsePhraseCountIntent(query, requesterUserId, botUserId);

    if (phraseCountIntent) {
      const count = await this.messageHistory.countPhrase(
        scope,
        phraseCountIntent.phrase,
        phraseCountIntent.subjectUserId
      );

      return {
        answer: `${phraseCountIntent.subjectLabel} said "${phraseCountIntent.phrase}" ${count} time${count === 1 ? '' : 's'} in ${scopeLabel(scope)}.`,
        source: 'exact'
      };
    }

    const recent = await this.loadRecentMessages(scope);
    const semanticMatches = await this.loadSemanticMatches(scope, query);
    const taskMatches = isTaskAwareQuery(query)
      ? await this.loadTaskMatches(requesterUserId)
      : [];
    const actionMatches = isHistoryAwareQuery(query)
      ? await this.loadActionMatches(requesterUserId, query)
      : [];

    if (semanticMatches.length === 0 && recent.length === 0 && actionMatches.length === 0 && taskMatches.length === 0) {
      if (!isHistoryAwareQuery(query) && !isTaskAwareQuery(query)) {
        const directAnswer = await this.answerDirect(query, []);
        return {
          answer: directAnswer,
          source: 'direct'
        };
      }

      return {
        answer: `I couldn't find enough history or open tasks in ${scopeLabel(scope)} to answer that yet.`,
        source: 'direct'
      };
    }

    const context = formatContext(recent, semanticMatches, actionMatches, taskMatches);
    const answer = await this.answerDirect(query, context.length > 0 ? [context] : []);

    return {
      answer,
      source: semanticMatches.length === 0 && recent.length === 0 && (actionMatches.length > 0 || taskMatches.length > 0)
        ? 'action'
        : 'semantic'
    };
  }

  private async answerDirect(query: string, contextBlocks: string[]): Promise<string> {
    try {
      return await this.responses.createTextResponse({
        model: this.env.OPENAI_RESPONSE_MODEL,
        instructions: [
          'You are GigiDC, a Discord assistant.',
          'Only describe the capabilities that actually exist in this bot runtime.',
          'Actual supported capabilities are DM chat, DM history recall, permitted guild-history recall, phrase counting, participant-visible task memory, participant-visible relay memory, task create/list/complete, and permission-gated DM relays.',
          'Do not claim to have web search, browsing, code execution, a sandbox, image generation, translation tools, or arbitrary external tool access.',
          'If chat history context is supplied, use it carefully.',
          'Be concise and practical.',
          'If the context is insufficient for a history-based question, say so plainly instead of guessing.'
        ].join(' '),
        text: contextBlocks.length > 0
          ? `Question: ${query}\n\nChat history context:\n${contextBlocks.join('\n\n')}`
          : `Question: ${query}`
      });
    } catch (error) {
      this.logger.error('OpenAI text response failed during retrieval', {
        error: error instanceof Error ? error.message : 'Unknown response-generation error',
        hasContext: contextBlocks.length > 0,
        query
      });

      return 'I could not reach my reasoning backend right now. Try again in a moment.';
    }
  }

  private async loadRecentMessages(scope: HistoryScope): Promise<HistoryMessageRecord[]> {
    try {
      return await this.messageHistory.listRecentMessages(scope, 6);
    } catch (error) {
      this.logger.warn('Recent message lookup failed during retrieval', {
        error: error instanceof Error ? error.message : 'Unknown recent-history error',
        scope: scope.kind
      });
      return [];
    }
  }

  private async loadSemanticMatches(scope: HistoryScope, query: string): Promise<HistoryMessageRecord[]> {
    try {
      return await this.messageHistory.searchSemantic(scope, query, 8);
    } catch (error) {
      this.logger.warn('Semantic search failed during retrieval', {
        error: error instanceof Error ? error.message : 'Unknown semantic-search error',
        query
      });
      return [];
    }
  }

  private async loadTaskMatches(requesterUserId: string): Promise<AgentActionRecord[]> {
    try {
      return await this.agentActions.listOpenTasksForUser(requesterUserId, 4);
    } catch (error) {
      this.logger.warn('Task memory lookup failed during retrieval', {
        error: error instanceof Error ? error.message : 'Unknown task-memory error',
        requesterUserId
      });
      return [];
    }
  }

  private async loadActionMatches(requesterUserId: string, query: string): Promise<AgentActionRecord[]> {
    try {
      return await this.agentActions.listRelevantVisibleActionsForUser(requesterUserId, query, 4);
    } catch (error) {
      this.logger.warn('Shared action lookup failed during retrieval', {
        error: error instanceof Error ? error.message : 'Unknown shared-action error',
        requesterUserId
      });
      return [];
    }
  }
}

function isTaskAwareQuery(query: string): boolean {
  return /\b(task|tasks|todo|to-do|supposed|assigned|assignment|deadline|due|follow up|follow-up|need to do|open task)\b/i.test(
    query
  );
}

function parseCapabilityIntent(query: string): string | null {
  const normalized = query.trim().toLowerCase();

  if (
    /\b(what tools can you call|what tools do you have|what can you do|what capabilities do you have|what are your capabilities)\b/i.test(
      normalized
    )
  ) {
    return [
      'In this bot runtime I can:',
      '- chat in DM',
      '- answer from your DM history',
      '- answer from permitted primary-server history when you have access',
      '- count exact phrases from stored history',
      '- recall participant-visible relays and tasks',
      '- create, list, and complete tasks',
      '- send Gigi-mediated DMs when permission checks pass',
      '',
      'I cannot browse the web, run code, provide a sandbox, generate images, or use arbitrary external tools here.'
    ].join('\n');
  }

  if (
    /\b(code execution|execute code|run code|sandbox|shell access|terminal access|browser|browse the web|web search|search the web|image generation|generate images|translation|translate)\b/i.test(
      normalized
    )
  ) {
    return 'No. In this bot runtime I cannot run code, provide a sandbox, browse the web, generate images, or use arbitrary external tools. The only internal tools I can use here are task create/list/complete and permission-gated DM relays.';
  }

  if (/\b(ingestion status|how.?s ingestion going|how is ingestion going|what.?s ingestion status)\b/i.test(normalized)) {
    return 'I do not have live ingestion-status reporting in DM. Use `/ingestion status` in a server channel. In DM I can only answer from stored DM history, permitted server history, and shared task/action memory.';
  }

  return null;
}

function isHistoryAwareQuery(query: string): boolean {
  return /\b(remember|history|chat|message|messages|said|mention|mentioned|talking|talked|discuss|discussed|asked|want|wanted|relay|again|last week|yesterday|before|server|guild)\b/i.test(
    query
  );
}

function parsePhraseCountIntent(
  query: string,
  requesterUserId: string,
  botUserId: string
): { phrase: string; subjectLabel: string; subjectUserId: string } | null {
  const match = query.match(
    /how many times did\s+(?<subject><@!?\d+>|i|me|you)\s+(?:say|mention|type)\s+["“](?<phrase>[^"”]+)["”]/i
  );

  const subject = match?.groups?.subject?.toLowerCase();
  const phrase = match?.groups?.phrase?.trim();

  if (!subject || !phrase) {
    return null;
  }

  if (subject === 'i' || subject === 'me') {
    return {
      phrase,
      subjectLabel: 'You',
      subjectUserId: requesterUserId
    };
  }

  if (subject === 'you') {
    return {
      phrase,
      subjectLabel: 'I',
      subjectUserId: botUserId
    };
  }

  const mentionedUserId = subject.replace(/[<@!>]/g, '');
  if (mentionedUserId.length === 0) {
    return null;
  }

  return {
    phrase,
    subjectLabel: `<@${mentionedUserId}>`,
    subjectUserId: mentionedUserId
  };
}

function formatContext(
  recent: HistoryMessageRecord[],
  semanticMatches: HistoryMessageRecord[],
  actionMatches: AgentActionRecord[],
  taskMatches: AgentActionRecord[]
): string {
  const sections: string[] = [];

  if (taskMatches.length > 0) {
    sections.push(
      'Open tasks:\n' +
        taskMatches
          .map((action) => formatTaskLine(action))
          .join('\n')
    );
  }

  if (actionMatches.length > 0) {
    sections.push(
      'Recent shared actions:\n' +
        actionMatches
          .map((action) => formatActionLine(action))
          .join('\n')
    );
  }

  if (recent.length > 0) {
    sections.push(
      'Recent messages:\n' +
        recent
          .map((message) => formatMessageLine(message))
          .join('\n')
    );
  }

  if (semanticMatches.length > 0) {
    sections.push(
      'Top semantic matches:\n' +
        semanticMatches
          .map((message) => formatMessageLine(message))
          .join('\n')
    );
  }

  return sections.join('\n\n');
}

function formatMessageLine(message: HistoryMessageRecord): string {
  const timestamp = new Date(message.created_at).toISOString();
  const content = message.content.length > 0 ? message.content : '[attachment only]';
  return `- [${timestamp}] ${message.author_username}: ${content}`;
}

function formatActionLine(action: AgentActionRecord): string {
  const timestamp = new Date(action.created_at).toISOString();
  const recipientLabel = action.recipient_username ?? 'unknown recipient';
  const parts = [
    `- [${timestamp}] ${action.requester_username} -> ${recipientLabel}`,
    `type=${action.action_type}`,
    `status=${action.status}`,
    `message="${action.instructions}"`
  ];

  if (typeof action.metadata.context === 'string' && action.metadata.context.trim().length > 0) {
    parts.push(`context="${action.metadata.context.trim()}"`);
  }

  if (action.result_summary) {
    parts.push(`result="${action.result_summary}"`);
  }

  if (action.error_message) {
    parts.push(`error="${action.error_message}"`);
  }

  return parts.join(' | ');
}

function formatTaskLine(action: AgentActionRecord): string {
  const timestamp = new Date(action.created_at).toISOString();
  const assigneeLabel = action.recipient_username ?? action.requester_username;
  const parts = [
    `- [${timestamp}] ${action.requester_username} assigned ${assigneeLabel}`,
    `status=${action.status}`,
    `title="${action.title}"`,
    `details="${action.instructions}"`
  ];

  if (action.due_at) {
    parts.push(`due="${new Date(action.due_at).toISOString()}"`);
  }

  return parts.join(' | ');
}

function scopeLabel(scope: HistoryScope): string {
  if (scope.kind === 'dm') {
    return 'this DM';
  }

  if (scope.channelId) {
    return `channel ${scope.channelId}`;
  }

  return 'the primary server';
}
