export type Scope = {
  gameId?: string;
  env?: string;
};

type ScopeListener = (scope: Scope) => void;

const STORAGE_KEYS = {
  gameId: 'game_id',
  env: 'env',
};

let currentScope: Scope = {};
let scopeReady = false;
let resolveScopeReady: () => void;
const listeners = new Set<ScopeListener>();

// Promise that resolves when GameSelector has validated the scope.
// API services can await this to avoid firing requests with stale values.
export const scopeReadyPromise = new Promise<void>((resolve) => {
  resolveScopeReady = resolve;
});

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
  return () => {
    listeners.delete(listener);
  };
};

// Initialize from storage on module load.
hydrateScope();

// If there is no auth token, the GameSelector will never load — resolve
// immediately so API calls are not blocked forever.
if (typeof window !== 'undefined' && !localStorage.getItem('token')) {
  markScopeReady();
}

// isScopeReady reports whether the GameSelector has finished its initial
// load and validated the scope (gameId + env). Until then, localStorage
// values may be stale and API calls should be deferred.
export function isScopeReady(): boolean {
  return scopeReady;
}

// markScopeReady signals that the GameSelector has completed its initial
// validation. It is called once after the first successful games load.
export function markScopeReady(): void {
  if (!scopeReady) {
    scopeReady = true;
    resolveScopeReady();
    emitScopeChange(currentScope);
  }
}
