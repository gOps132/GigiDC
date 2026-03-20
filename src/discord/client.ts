import { Client, Events, GatewayIntentBits } from 'discord.js';

import { commandMap } from './commands.js';
import type { BotContext } from './types.js';
import type { DiscordEventIngestionService } from '../services/discordEventIngestionService.js';

export function createDiscordClient(
  context: BotContext,
  ingestionService: DiscordEventIngestionService
): Client {
  const client = new Client({
    intents: [
      GatewayIntentBits.Guilds,
      GatewayIntentBits.GuildMessages,
      GatewayIntentBits.MessageContent
    ]
  });

  client.once(Events.ClientReady, (readyClient) => {
    context.logger.info('Discord client ready', {
      botUserId: readyClient.user.id,
      tag: readyClient.user.tag
    });
  });

  client.on(Events.InteractionCreate, async (interaction) => {
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
          ephemeral: true
        });
        return;
      }

      await interaction.reply({
        content: 'The command failed. Check the bot logs for details.',
        ephemeral: true
      });
    }
  });

  client.on(Events.MessageCreate, async (message) => {
    try {
      await ingestionService.ingestMessage(message);
    } catch (error) {
      const messageText = error instanceof Error ? error.message : 'Unknown error';
      context.logger.error('Discord message ingestion failed', {
        channelId: message.channelId,
        messageId: message.id,
        error: messageText
      });
    }
  });

  return client;
}
