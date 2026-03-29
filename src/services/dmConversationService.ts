import { randomUUID } from 'node:crypto';

import {
  ActionRowBuilder,
  StringSelectMenuBuilder,
  type Client,
  type Message,
  type StringSelectMenuInteraction,
  type User
} from 'discord.js';

import type { BotContext } from '../discord/types.js';
import type { PendingDmScopeSelectionStore, ScopeOption } from '../ports/conversation.js';
import { CAPABILITIES } from './rolePolicyService.js';
import { resolveDmScope, resolvePrimaryGuildScope, type HistoryScope } from './messageHistoryService.js';

const DM_SCOPE_SELECT_PREFIX = 'dm-scope';
const PENDING_SCOPE_TTL_MS = 15 * 60 * 1000;

export class DmConversationService {
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

      await message.reply({
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
      return;
    }

    const answer = await this.context.services.retrieval.answerQuestion(
      query,
      scopeOptions[0]?.scope ?? resolveDmScope(message.author),
      message.author.id,
      client.user?.id ?? ''
    );

    await message.reply({
      content: answer.answer
    });
  }

  matches(interaction: StringSelectMenuInteraction): boolean {
    return interaction.customId.startsWith(`${DM_SCOPE_SELECT_PREFIX}:`);
  }

  async handleSelection(interaction: StringSelectMenuInteraction, client: Client): Promise<void> {
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

    await interaction.followUp({
      content: answer.answer
    });
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
}

function shouldPromptForScope(query: string, scopeOptions: ScopeOption[]): boolean {
  if (scopeOptions.length <= 1) {
    return false;
  }

  return /\b(remember|history|chat|message|messages|said|mention|mentioned|talking|talked|discuss|discussed|last week|yesterday|before|server|guild)\b/i.test(
    query
  );
}
