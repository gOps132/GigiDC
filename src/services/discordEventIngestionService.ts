import type { Message } from 'discord.js';

import type { Logger } from '../lib/logger.js';
import type { ChannelIngestionPolicyService } from './channelIngestionPolicyService.js';
import type { ClawbotClient } from './clawbotClient.js';
import type { RolePolicyService } from './rolePolicyService.js';

export class DiscordEventIngestionService {
  constructor(
    private readonly policies: ChannelIngestionPolicyService,
    private readonly roles: RolePolicyService,
    private readonly clawbotClient: ClawbotClient,
    private readonly logger: Logger
  ) {}

  async ingestMessage(message: Message): Promise<void> {
    if (!message.inGuild() || message.author.bot) {
      return;
    }

    const guild = message.guild;
    await this.roles.ensureGuild(guild);

    const enabled = await this.policies.isEnabled(guild.id, message.channelId);
    if (!enabled) {
      return;
    }

    const attachmentMetadata = [...message.attachments.values()].map((attachment) => ({
      id: attachment.id,
      name: attachment.name ?? 'attachment',
      contentType: attachment.contentType,
      size: attachment.size,
      url: attachment.url
    }));

    await this.clawbotClient.ingestDiscordMessage({
      guildId: guild.id,
      channelId: message.channelId,
      threadId: message.channel.isThread() ? message.channel.id : null,
      messageId: message.id,
      authorId: message.author.id,
      authorUsername: message.author.username,
      content: message.content,
      createdAt: message.createdAt.toISOString(),
      attachmentMetadata
    });

    this.logger.debug('Forwarded Discord message to Clawbot', {
      guildId: guild.id,
      channelId: message.channelId,
      messageId: message.id,
      attachmentCount: attachmentMetadata.length
    });
  }
}
