import { randomUUID } from 'node:crypto';

import {
  ActionRowBuilder,
  StringSelectMenuBuilder,
  type Client,
  type Message,
  type ButtonInteraction,
  type StringSelectMenuInteraction,
  type User
} from 'discord.js';

import type { BotContext } from '../discord/types.js';
import type { PendingDmScopeSelectionStore, ScopeOption } from '../ports/conversation.js';
import { CAPABILITIES } from './rolePolicyService.js';
import { DmIntentRouter } from './dmIntentRouter.js';
import {
  resolveDmScope,
  resolveGuildChannelScope,
  resolvePrimaryGuildScope,
  type HistoryScope
} from './messageHistoryService.js';

const DM_SCOPE_SELECT_PREFIX = 'dm-scope';
const PENDING_SCOPE_TTL_MS = 15 * 60 * 1000;

export class DmConversationService {
  private readonly intentRouter = new DmIntentRouter();

  constructor(
    private readonly context: BotContext,
    private readonly pendingSelections: PendingDmScopeSelectionStore
  ) {}

  async handleMessage(message: Message, client: Client): Promise<void> {
    if (!message.channel.isDMBased() || message.author.bot) {
      return;
    }

    const query = message.content.trim();
    if (query.length === 0) {
      return;
    }

    const primaryGuild = await this.resolvePrimaryGuild(client);
    const primaryMember = primaryGuild
      ? await primaryGuild.members.fetch(message.author.id).catch(() => null)
      : null;
    await this.context.services.userMemory.syncProfile({
      displayName: primaryMember?.displayName ?? message.author.globalName ?? null,
      guildId: primaryGuild?.id ?? null,
      user: message.author
    });

    const sensitiveReply = await this.context.services.sensitiveData.maybeHandleDmQuery(
      query,
      message.author,
      client
    );
    if (sensitiveReply) {
      await message.reply({
        content: sensitiveReply.reply
      });

      return;
    }

    const route = this.intentRouter.route(query);
    if (route.kind === 'direct_reply') {
      const reply = await message.reply({
        content: route.reply
      });
      await this.captureOutboundReply(reply, 'dm tool reply');
      return;
    }

    if (route.kind === 'confirm_pending_action' || route.kind === 'cancel_pending_action') {
      const confirmation = await this.context.services.actionConfirmations.maybeHandleTextConfirmation(
        query,
        message.author,
        client
      );

      if (confirmation) {
        const reply = await message.reply({
          content: confirmation.reply
        });
        await this.captureOutboundReply(reply, 'dm action confirmation reply');
        return;
      }
    }

    if (route.kind === 'tool_request') {
      const toolResult = await this.context.services.agentTools.maybeHandleDmQuery(
        query,
        message.author,
        client,
        message.channelId,
        {
          mentionedUsers: [...message.mentions.users.values()].filter((user) => user.id !== client.user?.id)
        }
      );
      if (toolResult) {
        const reply = await message.reply({
          content: toolResult.reply,
          components: toolResult.components
        });
        await this.captureOutboundReply(reply, 'dm tool reply');
        return;
      }
    }

    const scopeOptions = await this.resolveAvailableScopes(client, message.author);

    if (shouldPromptForScope(query, scopeOptions)) {
      const selectionId = randomUUID();
      await this.pendingSelections.deleteExpired(new Date());
      await this.pendingSelections.save({
        createdAt: Date.now(),
        id: selectionId,
        query,
        scopeOptions,
        userId: message.author.id
      }, new Date(Date.now() + PENDING_SCOPE_TTL_MS));

      const prompt = await message.reply({
        content: 'Pick which chat history I should use for this question.',
        components: [
          new ActionRowBuilder<StringSelectMenuBuilder>().addComponents(
            new StringSelectMenuBuilder()
              .setCustomId(`${DM_SCOPE_SELECT_PREFIX}:${selectionId}`)
              .setPlaceholder('Choose a history scope')
              .addOptions(
                scopeOptions.map((option) => ({
                  label: option.label,
                  value: option.value
                }))
              )
          )
        ]
      });
      await this.captureOutboundReply(prompt, 'dm scope prompt');
      return;
    }

    const answer = await this.context.services.retrieval.answerQuestion(
      query,
      scopeOptions[0]?.scope ?? resolveDmScope(message.author),
      message.author.id,
      client.user?.id ?? ''
    );

    const reply = await message.reply({
      content: answer.answer
    });
    await this.captureOutboundReply(reply, 'dm direct reply');
  }

  shouldHandleGuildMention(message: Message, client: Client): boolean {
    if (!message.inGuild() || message.author.bot) {
      return false;
    }

    const botUserId = client.user?.id;
    if (!botUserId) {
      return false;
    }

    return message.mentions.users.has(botUserId);
  }

  async handleGuildMention(message: Message, client: Client): Promise<void> {
    if (!message.inGuild() || message.author.bot || !client.user) {
      return;
    }

    const query = normalizeGuildMentionQuery(message.content, client.user.id);
    if (query.length === 0) {
      const reply = await message.reply({
        content: 'Hi. Ask me about this channel, or DM me for private actions.'
      });
      await this.captureOutboundReply(reply, 'guild mention greeting');
      return;
    }

    await this.context.services.userMemory.syncProfile({
      displayName: message.member?.displayName ?? message.author.globalName ?? null,
      guildId: message.guildId,
      user: message.author
    });

    const route = this.intentRouter.route(query);
    if (route.kind === 'direct_reply') {
      const reply = await message.reply({
        content: route.reply
      });
      await this.captureOutboundReply(reply, 'guild mention direct reply');
      return;
    }

    if (
      route.kind === 'tool_request'
      || route.kind === 'confirm_pending_action'
      || route.kind === 'cancel_pending_action'
    ) {
      const reply = await message.reply({
        content: 'I can answer from this channel when you mention me here, but for tasks, relays, permissions, admin actions, confirmations, or sensitive-data flows, DM me or use the matching slash command.'
      });
      await this.captureOutboundReply(reply, 'guild mention tool redirect');
      return;
    }

    const answer = await this.context.services.retrieval.answerQuestion(
      query,
      resolveGuildChannelScope(message.guildId, message.channelId),
      message.author.id,
      client.user.id,
      {
        includeParticipantMemory: false,
        includeUserMemory: false
      }
    );

    const reply = await message.reply({
      content: answer.answer
    });
    await this.captureOutboundReply(reply, 'guild mention reply');
  }

  matches(interaction: StringSelectMenuInteraction): boolean {
    return interaction.customId.startsWith(`${DM_SCOPE_SELECT_PREFIX}:`)
      || this.context.services.agentTools.matchesRecipientSelection(interaction);
  }

  matchesButton(interaction: ButtonInteraction): boolean {
    return this.context.services.actionConfirmations.matches(interaction);
  }

  async handleSelection(interaction: StringSelectMenuInteraction, client: Client): Promise<void> {
    if (this.context.services.agentTools.matchesRecipientSelection(interaction)) {
      await this.context.services.agentTools.handleRecipientSelection(interaction, client);
      return;
    }

    const selectionId = interaction.customId.replace(`${DM_SCOPE_SELECT_PREFIX}:`, '');
    const pending = await this.pendingSelections.get(selectionId);

    if (!pending || Date.now() - pending.createdAt > PENDING_SCOPE_TTL_MS) {
      await this.pendingSelections.delete(selectionId);
      await interaction.reply({
        content: 'That scope selection has expired. Ask me again and I will re-run it.'
      });
      return;
    }

    if (pending.userId !== interaction.user.id) {
      await interaction.reply({
        content: 'That scope menu belongs to another user.'
      });
      return;
    }

    const option = pending.scopeOptions.find((scopeOption) => scopeOption.value === interaction.values[0]);

    if (!option) {
      await interaction.reply({
        content: 'That scope option was not recognized.'
      });
      return;
    }

    await this.pendingSelections.delete(selectionId);

    await interaction.update({
      content: `Using ${option.label.toLowerCase()} for your question.`,
      components: []
    });

    const answer = await this.context.services.retrieval.answerQuestion(
      pending.query,
      option.scope,
      interaction.user.id,
      client.user?.id ?? ''
    );

    const followUp = await interaction.followUp({
      content: answer.answer
    });
    await this.captureOutboundReply(followUp as Message, 'dm scope follow-up');
  }

  async handleActionButton(interaction: ButtonInteraction, client: Client): Promise<void> {
    await this.context.services.actionConfirmations.handleButton(interaction, client);
  }

  private async resolveAvailableScopes(client: Client, user: User): Promise<ScopeOption[]> {
    const scopes: ScopeOption[] = [
      {
        label: 'This DM',
        value: 'dm',
        scope: resolveDmScope(user)
      }
    ];

    const primaryGuildId = this.context.env.PRIMARY_GUILD_ID ?? this.context.env.DISCORD_GUILD_ID;
    if (!primaryGuildId) {
      return scopes;
    }

    const guild = client.guilds.cache.get(primaryGuildId) ?? (await client.guilds.fetch(primaryGuildId).catch(() => null));
    if (!guild) {
      return scopes;
    }

    const member = await guild.members.fetch(user.id).catch(() => null);
    if (!member) {
      return scopes;
    }

    const allowed = await this.context.services.rolePolicies.memberHasCapability(
      guild,
      member,
      CAPABILITIES.historyGuildWide
    );

    if (!allowed) {
      return scopes;
    }

    scopes.push({
      label: `${guild.name} server`,
      value: `guild:${guild.id}`,
      scope: resolvePrimaryGuildScope(guild.id)
    });

    return scopes;
  }

  private async resolvePrimaryGuild(client: Client) {
    const primaryGuildId = this.context.env.PRIMARY_GUILD_ID ?? this.context.env.DISCORD_GUILD_ID;
    if (!primaryGuildId) {
      return null;
    }

    return client.guilds.cache.get(primaryGuildId)
      ?? (await client.guilds.fetch(primaryGuildId).catch(() => null));
  }

  private async captureOutboundReply(message: Message, label: string): Promise<void> {
    try {
      await this.context.services.messageHistory.storeBotAuthoredMessage(message);
    } catch (error) {
      const messageText = error instanceof Error ? error.message : 'Unknown DM reply persistence error';
      this.context.logger.error('Failed to persist outbound bot-authored DM message', {
        error: messageText,
        label,
        messageId: message.id
      });
    }
  }
}

function shouldPromptForScope(query: string, scopeOptions: ScopeOption[]): boolean {
  if (scopeOptions.length <= 1) {
    return false;
  }

  return /\b(remember|history|chat|message|messages|said|mention|mentioned|talking|talked|discuss|discussed|asked|want|wanted|relay|again|last week|yesterday|before|server|guild)\b/i.test(
    query
  );
}

function normalizeGuildMentionQuery(content: string, botUserId: string): string {
  const mentionPattern = new RegExp(`<@!?${botUserId}>`, 'g');
  return content.replace(mentionPattern, '').trim();
}
