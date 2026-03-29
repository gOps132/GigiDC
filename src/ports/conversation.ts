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
