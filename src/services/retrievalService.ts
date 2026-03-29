import type { Env } from '../config/env.js';
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
    private readonly agentActions: AgentActionService
  ) {}

  async answerQuestion(
    query: string,
    scope: HistoryScope,
    requesterUserId: string,
    botUserId: string
  ): Promise<RetrievalAnswer> {
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

    const recent = await this.messageHistory.listRecentMessages(scope, 6);
    const semanticMatches = await this.messageHistory.searchSemantic(scope, query, 8);
    const taskMatches = isTaskAwareQuery(query)
      ? await this.agentActions.listOpenTasksForUser(requesterUserId, 4)
      : [];
    const actionMatches = isHistoryAwareQuery(query)
      ? await this.agentActions.listRelevantVisibleActionsForUser(requesterUserId, query, 4)
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
    return this.responses.createTextResponse({
      model: this.env.OPENAI_RESPONSE_MODEL,
      instructions: [
        'You are GigiDC, a Discord assistant.',
        'If chat history context is supplied, use it carefully.',
        'Be concise and practical.',
        'If the context is insufficient for a history-based question, say so plainly instead of guessing.'
      ].join(' '),
      text: contextBlocks.length > 0
        ? `Question: ${query}\n\nChat history context:\n${contextBlocks.join('\n\n')}`
        : `Question: ${query}`
    });
  }
}

function isTaskAwareQuery(query: string): boolean {
  return /\b(task|tasks|todo|to-do|supposed|assigned|assignment|deadline|due|follow up|follow-up|need to do|open task)\b/i.test(
    query
  );
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
