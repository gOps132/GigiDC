import { REST, Routes } from 'discord.js';

import { loadEnv } from '../config/env.js';
import { commands } from './commands.js';

export async function registerApplicationCommands(): Promise<void> {
  const env = loadEnv();
  const rest = new REST({ version: '10' }).setToken(env.DISCORD_TOKEN);
  const body = commands.map((command) => command.data.toJSON());

  if (env.DISCORD_GUILD_ID) {
    await rest.put(
      Routes.applicationGuildCommands(env.DISCORD_CLIENT_ID, env.DISCORD_GUILD_ID),
      { body }
    );
    return;
  }

  await rest.put(Routes.applicationCommands(env.DISCORD_CLIENT_ID), { body });
}

if (import.meta.url === `file://${process.argv[1]}`) {
  registerApplicationCommands()
    .then(() => {
      console.log('Registered application commands.');
    })
    .catch((error) => {
      console.error('Failed to register application commands.', error);
      process.exitCode = 1;
    });
}
