type LogLevel = 'debug' | 'info' | 'warn' | 'error';

const levelWeights: Record<LogLevel, number> = {
  debug: 10,
  info: 20,
  warn: 30,
  error: 40
};

export class Logger {
  constructor(private readonly minimumLevel: LogLevel = 'info') {}

  debug(message: string, meta?: Record<string, unknown>): void {
    this.write('debug', message, meta);
  }

  info(message: string, meta?: Record<string, unknown>): void {
    this.write('info', message, meta);
  }

  warn(message: string, meta?: Record<string, unknown>): void {
    this.write('warn', message, meta);
  }

  error(message: string, meta?: Record<string, unknown>): void {
    this.write('error', message, meta);
  }

  private write(level: LogLevel, message: string, meta?: Record<string, unknown>): void {
    if (levelWeights[level] < levelWeights[this.minimumLevel]) {
      return;
    }

    const payload = {
      level,
      message,
      ...meta,
      timestamp: new Date().toISOString()
    };

    console.log(JSON.stringify(payload));
  }
}
