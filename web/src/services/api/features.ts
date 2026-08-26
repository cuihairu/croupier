import { request } from '@umijs/max';

// Source: internal/api/meta/dto.go RootResponse (public endpoint, no auth).
type RootResponse = {
  service?: string;
  version?: string;
  features?: string[];
  profiles?: string[];
};

export type ServerFeatures = {
  dev: boolean;
  support: boolean;
  analytics: boolean;
  ops: boolean;
  extensions: boolean;
};

// All domains enabled — the fail-open fallback when the meta endpoint cannot
// be reached (a broken meta API must not lock users out of the product).
export const ALL_FEATURES_ON: ServerFeatures = {
  dev: true,
  support: true,
  analytics: true,
  ops: true,
  extensions: true,
};

/**
 * Fetches the enabled product domains from GET /api/v1 (server featureFlags).
 * Domains absent from the response are disabled; on any transport failure all
 * domains default to enabled (fail-open, mirroring the backend's
 * FeatureFlagsConfig.Enabled semantics).
 */
export async function fetchServerFeatures(): Promise<ServerFeatures> {
  try {
    const root = await request<RootResponse>('/api/v1', {
      method: 'GET',
      skipErrorHandler: true,
    });
    const list = new Set(root?.features || []);
    if (list.size === 0) return { ...ALL_FEATURES_ON };
    const on = (key: keyof ServerFeatures) => list.has(key);
    return {
      dev: on('dev'),
      support: on('support'),
      analytics: on('analytics'),
      ops: on('ops'),
      extensions: on('extensions'),
    };
  } catch {
    return { ...ALL_FEATURES_ON };
  }
}
