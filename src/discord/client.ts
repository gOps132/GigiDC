import { Client, Events, GatewayIntentBits, MessageFlags, Partials, type Message } from 'discord.js';

import { commandMap } from './commands.js';
import type { BotContext } from './types.js';

export function createDiscordClient(context: BotContext): Client {
  const client = new Client({
    intents: [
      GatewayIntentBits.Guilds,
      GatewayIntentBits.GuildMessages,
      GatewayIntentBits.DirectMessages,
      GatewayIntentBits.MessageContent
    ],
    partials: [Partials.Channel]
  });

  client.once(Events.ClientReady, (readyClient) => {
    context.runtime.markDiscordReady(readyClient.user.id);
    context.logger.info('Discord client ready', {
      botUserId: readyClient.user.id,
      tag: readyClient.user.tag
    });
  });

  client.on(Events.InteractionCreate, async (interaction) => {
    if (interaction.isButton() && context.services.dmConversation.matchesButton(interaction)) {
      try {
        await context.services.dmConversation.handleActionButton(interaction, client);
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Unknown error';
        context.logger.error('Button interaction failed', {
          customId: interaction.customId,
          error: message
        });

        if (interaction.replied || interaction.deferred) {
          await interaction.followUp({
            content: 'That action failed. Please ask again.',
            flags: MessageFlags.Ephemeral
          });
          return;
        }

        await interaction.reply({
          content: 'That action failed. Please ask again.',
          flags: MessageFlags.Ephemeral
        });
      }
      return;
    }

    if (interaction.isStringSelectMenu() && context.services.dmConversation.matches(interaction)) {
      try {
        await context.services.dmConversation.handleSelection(interaction, client);
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Unknown error';
        context.logger.error('Select menu interaction failed', {
          customId: interaction.customId,
          error: message
        });

        if (interaction.replied || interaction.deferred) {
          await interaction.followUp({
            content: 'That selection failed. Please ask again.',
            flags: MessageFlags.Ephemeral
          });
          return;
        }

        await interaction.reply({
          content: 'That selection failed. Please ask again.',
          flags: MessageFlags.Ephemeral
        });
      }
      return;
    }

    if (!interaction.isChatInputCommand()) {
      return;
    }

    const command = commandMap.get(interaction.commandName);

    if (!command) {
      await interaction.reply({
        content: 'Unknown command.',
        ephemeral: true
      });
      return;
    }

    try {
      await command.execute(interaction, context);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown error';
      context.logger.error('Command execution failed', {
        commandName: interaction.commandName,
        error: message
      });

      if (interaction.deferred || interaction.replied) {
        await interaction.followUp({
          content: 'The command failed. Check the bot logs for details.',
          flags: MessageFlags.Ephemeral
        });
        return;
      }

      await interaction.reply({
        content: 'The command failed. Check the bot logs for details.',
        flags: MessageFlags.Ephemeral
      });
    }
  });

  client.on(Events.MessageCreate, async (message) => {
    await handleIncomingDiscordMessage(message, client, context);
  });

  client.on(Events.ShardDisconnect, () => {
    context.runtime.markDiscordDegraded();
  });

  return client;
}

export async function handleIncomingDiscordMessage(
  message: Message,
  client: Client,
  context: BotContext
): Promise<void> {
  const isGuildMention = context.services.dmConversation.shouldHandleGuildMention(message, client);
  let storeSucceeded = true;
  const skipHistoryStorage =
    message.channel.isDMBased()
    && !message.author.bot
    && context.services.sensitiveData.shouldBypassHistoryStorage(message.content);

  try {
    if (!skipHistoryStorage) {
      const result = await context.services.messageHistory.storeDiscordMessage(message);

      if (
        message.inGuild()
        && !result.stored
        && result.reason === 'skipped_by_ingestion_policy'
        && !isGuildMention
      ) {
        return;
      }
    }
  } catch (error) {
    storeSucceeded = false;
    const messageText = error instanceof Error ? error.message : 'Unknown message storage error';
    context.logger.error('Discord message storage failed', {
      channelId: message.channelId,
      messageId: message.id,
      error: messageText
    });

    if ((message.inGuild() && !isGuildMention) || message.author.bot) {
      return;
    }
  }

  if (message.author.bot) {
    return;
  }

  if (message.channel.isDMBased()) {
    try {
      await context.services.dmConversation.handleMessage(message, client);
    } catch (error) {
      const messageText = error instanceof Error ? error.message : 'Unknown DM conversation error';
      context.logger.error('Discord DM handling failed', {
        channelId: message.channelId,
        messageId: message.id,
        error: messageText,
        historyStored: !skipHistoryStorage && storeSucceeded
      });

      await message.reply({
        content: 'I hit an internal error while handling that DM. Try again in a moment.'
      }).catch(() => undefined);
    }
    return;
  }

  if (!isGuildMention) {
    return;
  }

  try {
    await context.services.dmConversation.handleGuildMention(message, client);
  } catch (error) {
    const messageText = error instanceof Error ? error.message : 'Unknown guild mention conversation error';
    context.logger.error('Discord guild mention handling failed', {
      channelId: message.channelId,
      messageId: message.id,
      error: messageText,
      historyStored: storeSucceeded
    });

    await message.reply({
      content: 'I hit an internal error while handling that mention. Try again in a moment.'
    }).catch(() => undefined);
  }
}
