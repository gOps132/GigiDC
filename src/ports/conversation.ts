import type { HistoryScope } from './history.js';

export interface ScopeOption {
  label: string;
  value: string;
  scope: HistoryScope;
}

export interface PendingDmScopeSelection {
  createdAt: number;
  id: string;
  query: string;
  scopeOptions: ScopeOption[];
  userId: string;
}

export interface PendingDmScopeSelectionStore {
  delete(selectionId: string): Promise<void>;
  deleteExpired(now: Date): Promise<void>;
  get(selectionId: string): Promise<PendingDmScopeSelection | null>;
  save(selection: PendingDmScopeSelection, expiresAt: Date): Promise<void>;
}

export interface PendingDmRelayRecipientOption {
  displayLabel: string;
  username: string;
  userId: string;
}

export interface PendingDmRelayRecipientSelection {
  channelId: string;
  createdAt: number;
  guildId: string | null;
  id: string;
  recipientOptions: PendingDmRelayRecipientOption[];
  relayContext: string | null;
  relayMessage: string;
  requesterUserId: string;
  requesterUsername: string;
}

export interface PendingDmRelayRecipientSelectionStore {
  delete(selectionId: string): Promise<void>;
  deleteExpired(now: Date): Promise<void>;
  get(selectionId: string): Promise<PendingDmRelayRecipientSelection | null>;
  save(selection: PendingDmRelayRecipientSelection, expiresAt: Date): Promise<void>;
}
