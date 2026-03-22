import type OpenAI from 'openai';

import type { Env } from '../config/env.js';
import type { HistoryMessageRecord, HistoryScope, MessageHistoryService } from './messageHistoryService.js';

export interface RetrievalAnswer {
  answer: string;
  source: 'exact' | 'semantic' | 'direct';
}

export class RetrievalService {
  constructor(
    private readonly env: Env,
    private readonly openai: OpenAI,
    private readonly messageHistory: MessageHistoryService
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

    if (semanticMatches.length === 0 && recent.length === 0) {
      if (!isHistoryAwareQuery(query)) {
        const directAnswer = await this.answerDirect(query, []);
        return {
          answer: directAnswer,
          source: 'direct'
        };
      }

      return {
        answer: `I couldn't find enough history in ${scopeLabel(scope)} to answer that yet.`,
        source: 'direct'
      };
    }

    const context = formatContext(recent, semanticMatches);
    const answer = await this.answerDirect(query, context.length > 0 ? [context] : []);

    return {
      answer,
      source: 'semantic'
    };
  }

  private async answerDirect(query: string, contextBlocks: string[]): Promise<string> {
    const response = await this.openai.responses.create({
      model: this.env.OPENAI_RESPONSE_MODEL,
      instructions: [
        'You are GigiDC, a Discord assistant.',
        'If chat history context is supplied, use it carefully.',
        'Be concise and practical.',
        'If the context is insufficient for a history-based question, say so plainly instead of guessing.'
      ].join(' '),
      input: [
        {
          role: 'user',
          content: [
            {
              type: 'input_text',
              text: contextBlocks.length > 0
                ? `Question: ${query}\n\nChat history context:\n${contextBlocks.join('\n\n')}`
                : `Question: ${query}`
            }
          ]
        }
      ]
    });

    return response.output_text.trim() || 'I could not produce a useful answer for that yet.';
  }
}

function isHistoryAwareQuery(query: string): boolean {
  return /\b(remember|history|chat|message|messages|said|mention|mentioned|talking|talked|discuss|discussed|last week|yesterday|before|server|guild)\b/i.test(
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
  semanticMatches: HistoryMessageRecord[]
): string {
  const sections: string[] = [];

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

function scopeLabel(scope: HistoryScope): string {
  if (scope.kind === 'dm') {
    return 'this DM';
  }

  if (scope.channelId) {
    return `channel ${scope.channelId}`;
  }

  return 'the primary server';
}
