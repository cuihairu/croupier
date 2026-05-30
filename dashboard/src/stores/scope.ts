type Scope = {
  gameId?: string;
  env?: string;
};

type ScopeListener = (scope: Scope) => void;

const STORAGE_KEYS = {
  gameId: 'game_id',
  env: 'env',
};

let currentScope: Scope = {};
const listeners = new Set<ScopeListener>();

const readFromStorage = (): Scope => {
  if (typeof window === 'undefined') return {};
  try {
    return {
      gameId: localStorage.getItem(STORAGE_KEYS.gameId) || undefined,
      env: localStorage.getItem(STORAGE_KEYS.env) || undefined,
    };
  } catch {
    return {};
  }
};

const persistToStorage = (scope: Scope) => {
  if (typeof window === 'undefined') return;
  try {
    if (scope.gameId) {
      localStorage.setItem(STORAGE_KEYS.gameId, scope.gameId);
    }
    if (scope.env) {
      localStorage.setItem(STORAGE_KEYS.env, scope.env);
    }
  } catch {
    // ignore persistence errors
  }
};

const emitScopeChange = (scope: Scope) => {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('scope:change', { detail: scope }));
  }
  listeners.forEach((listener) => listener(scope));
};

export const getScope = (): Scope => ({ ...currentScope });

export const setScope = (next: Scope, opts?: { persist?: boolean; emit?: boolean }) => {
  currentScope = {
    ...currentScope,
    ...next,
  };
  if (opts?.persist !== false) {
    persistToStorage(currentScope);
  }
  if (opts?.emit !== false) {
    emitScopeChange(currentScope);
  }
  return currentScope;
};

export const hydrateScope = () => {
  const stored = readFromStorage();
  if (stored.gameId || stored.env) {
    setScope(stored, { persist: false });
  }
  return getScope();
};

export const subscribeScope = (listener: ScopeListener) => {
  listeners.add(listener);
  return () => listeners.delete(listener);
};

// Initialize from storage on module load.
hydrateScope();
