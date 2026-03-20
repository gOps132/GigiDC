import { assignmentCommand } from '../commands/assignment.js';
import { generateCommand } from '../commands/generate.js';
import { ingestionCommand } from '../commands/ingestion.js';
import { notesCommand } from '../commands/notes.js';
import { pingCommand } from '../commands/ping.js';
import { reviewCommand } from '../commands/review.js';
import type { SlashCommand } from './types.js';

export const commands: SlashCommand[] = [
  pingCommand,
  assignmentCommand,
  ingestionCommand,
  reviewCommand,
  generateCommand,
  notesCommand
];

export const commandMap = new Map(commands.map((command) => [command.data.name, command]));
