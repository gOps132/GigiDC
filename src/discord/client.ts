import { Client, Events, GatewayIntentBits, MessageFlags, Partials } from 'discord.js';

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
    context.logger.info('Discord client ready', {
      botUserId: readyClient.user.id,
      tag: readyClient.user.tag
    });
  });

  client.on(Events.InteractionCreate, async (interaction) => {
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
    try {
      await context.services.messageHistory.storeDiscordMessage(message);

      if (message.channel.isDMBased() && !message.author.bot) {
        await context.services.dmConversation.handleMessage(message, client);
      }
    } catch (error) {
      const messageText = error instanceof Error ? error.message : 'Unknown error';
      context.logger.error('Discord message handling failed', {
        channelId: message.channelId,
        messageId: message.id,
        error: messageText
      });
    }
  });

  return client;
}
