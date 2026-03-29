import type { MessageIndexingStatus } from './messageIndexingService.js';

export interface RuntimeSnapshot {
  checks: {
    commandRegistration: {
      error: string | null;
      status: 'pending' | 'ready' | 'failed' | 'skipped';
      updatedAt: string | null;
    };
    discordGateway: {
      botUserId: string | null;
      status: 'starting' | 'ready' | 'degraded';
      updatedAt: string | null;
    };
    messageIndexing: MessageIndexingStatus;
  };
  ready: boolean;
  startedAt: string;
}

type CommandRegistrationStatus = 'pending' | 'ready' | 'failed' | 'skipped';
type DiscordGatewayStatus = 'starting' | 'ready' | 'degraded';

export class RuntimeStateService {
  private commandRegistrationError: string | null = null;
  private commandRegistrationStatus: CommandRegistrationStatus = 'pending';
  private commandRegistrationUpdatedAt: string | null = null;
  private discordBotUserId: string | null = null;
  private discordGatewayStatus: DiscordGatewayStatus = 'starting';
  private discordGatewayUpdatedAt: string | null = null;
  private readonly startedAt = new Date().toISOString();

  markCommandRegistrationReady(): void {
    this.commandRegistrationStatus = 'ready';
    this.commandRegistrationError = null;
    this.commandRegistrationUpdatedAt = new Date().toISOString();
  }

  markCommandRegistrationFailed(error: string): void {
    this.commandRegistrationStatus = 'failed';
    this.commandRegistrationError = error;
    this.commandRegistrationUpdatedAt = new Date().toISOString();
  }

  markCommandRegistrationSkipped(): void {
    this.commandRegistrationStatus = 'skipped';
    this.commandRegistrationError = null;
    this.commandRegistrationUpdatedAt = new Date().toISOString();
  }

  markDiscordReady(botUserId: string | null): void {
    this.discordGatewayStatus = 'ready';
    this.discordBotUserId = botUserId;
    this.discordGatewayUpdatedAt = new Date().toISOString();
  }

  markDiscordDegraded(): void {
    this.discordGatewayStatus = 'degraded';
    this.discordGatewayUpdatedAt = new Date().toISOString();
  }

  getSnapshot(messageIndexing: MessageIndexingStatus): RuntimeSnapshot {
    const ready =
      this.discordGatewayStatus === 'ready' &&
      (this.commandRegistrationStatus === 'ready' || this.commandRegistrationStatus === 'skipped');

    return {
      checks: {
        commandRegistration: {
          error: this.commandRegistrationError,
          status: this.commandRegistrationStatus,
          updatedAt: this.commandRegistrationUpdatedAt
        },
        discordGateway: {
          botUserId: this.discordBotUserId,
          status: this.discordGatewayStatus,
          updatedAt: this.discordGatewayUpdatedAt
        },
        messageIndexing
      },
      ready,
      startedAt: this.startedAt
    };
  }
}
