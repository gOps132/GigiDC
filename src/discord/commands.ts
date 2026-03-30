import { assignmentCommand } from '../commands/assignment.js';
import { heheCommand } from '../commands/hehe.js';
import { ingestionCommand } from '../commands/ingestion.js';
import { pingCommand } from '../commands/ping.js';
import { permissionCommand } from '../commands/permission.js';
import { relayCommand } from '../commands/relay.js';
import { taskCommand } from '../commands/task.js';
import { usageCommand } from '../commands/usage.js';
import type { SlashCommand } from './types.js';

export const commands: SlashCommand[] = [
  pingCommand,
  heheCommand,
  permissionCommand,
  usageCommand,
  ingestionCommand,
  assignmentCommand,
  relayCommand,
  taskCommand
];

export const commandMap = new Map(commands.map((command) => [command.data.name, command]));
