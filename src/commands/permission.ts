import {
  EmbedBuilder,
  MessageFlags,
  SlashCommandBuilder
} from 'discord.js';

import type { BotContext, SlashCommand } from '../discord/types.js';
import { ALL_CAPABILITIES } from '../services/rolePolicyService.js';

const permissionCommandData = new SlashCommandBuilder()
  .setName('permission')
  .setDescription('Manage direct user capabilities for Gigi.')
  .addSubcommand((subcommand) =>
    subcommand
      .setName('grant')
      .setDescription('Grant a direct Gigi capability to a user.')
      .addUserOption((option) =>
        option
          .setName('user')
          .setDescription('The user receiving the direct grant.')
          .setRequired(true)
      )
      .addStringOption((option) => {
        let withChoices = option
          .setName('capability')
          .setDescription('The capability to grant.')
          .setRequired(true);

        for (const capability of ALL_CAPABILITIES) {
          withChoices = withChoices.addChoices({
            name: capability,
            value: capability
          });
        }

        return withChoices;
      })
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('revoke')
      .setDescription('Revoke a direct Gigi capability from a user.')
      .addUserOption((option) =>
        option
          .setName('user')
          .setDescription('The user losing the direct grant.')
          .setRequired(true)
      )
      .addStringOption((option) => {
        let withChoices = option
          .setName('capability')
          .setDescription('The capability to revoke.')
          .setRequired(true);

        for (const capability of ALL_CAPABILITIES) {
          withChoices = withChoices.addChoices({
            name: capability,
            value: capability
          });
        }

        return withChoices;
      })
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('list')
      .setDescription('List a user’s effective and direct Gigi capabilities.')
      .addUserOption((option) =>
        option
          .setName('user')
          .setDescription('The user to inspect. Defaults to you.')
      )
  );

export const permissionCommand: SlashCommand = {
  data: permissionCommandData,
  async execute(interaction, context) {
    if (!interaction.inGuild()) {
      await interaction.reply({
        content: 'This command can only be used in a server.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const guild = interaction.guild;
    if (!guild) {
      await interaction.reply({
        content: 'Guild context was not available for this command.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const subcommand = interaction.options.getSubcommand();
    const targetUser = interaction.options.getUser('user') ?? interaction.user;

    if (subcommand === 'list') {
      const summary = await context.services.permissionAdmin.listUserPermissions({
        client: interaction.client,
        requester: interaction.user,
        targetUser
      });

      await interaction.reply({
        embeds: [
          new EmbedBuilder()
            .setTitle('Permission summary')
            .setDescription(summary)
            .setColor(0x5865f2)
        ],
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const capability = interaction.options.getString('capability', true);
    const summary = subcommand === 'grant'
      ? await context.services.permissionAdmin.grantUserPermission({
          capability,
          client: interaction.client,
          requester: interaction.user,
          targetUser
        })
      : await context.services.permissionAdmin.revokeUserPermission({
          capability,
          client: interaction.client,
          requester: interaction.user,
          targetUser
        });

    await interaction.reply({
      content: summary,
      flags: MessageFlags.Ephemeral
    });
  }
};
