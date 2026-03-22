import { assignmentCommand } from '../commands/assignment.js';
import { heheCommand } from '../commands/hehe.js';
import { pingCommand } from '../commands/ping.js';
import type { SlashCommand } from './types.js';

export const commands: SlashCommand[] = [
  pingCommand,
  heheCommand,
  assignmentCommand
];

export const commandMap = new Map(commands.map((command) => [command.data.name, command]));
