import type {
  ExtensionBindingItem,
  ExtensionCatalogItem,
  ExtensionEventItem,
  ExtensionInstallationItem,
  ExtensionReleaseItem,
} from '@/services/api/extensions';

type Maybe<T> = T | null | undefined;

function toArray<T>(value: Maybe<T[]>, fallback: T[] = []): T[] {
  return Array.isArray(value) ? value : fallback;
}

export function adaptCatalogListResponse(
  resp: Maybe<{ items?: ExtensionCatalogItem[]; total?: number }>,
) {
  return {
    items: toArray(resp?.items, []),
    total: Number(resp?.total || 0),
  };
}

export function adaptCatalogDetailResponse(
  resp: Maybe<{
    item?: ExtensionCatalogItem;
    releases?: ExtensionReleaseItem[];
    capabilities?: string[];
  }>,
  fallbackItem?: ExtensionCatalogItem,
) {
  return {
    item: (resp?.item || fallbackItem) as ExtensionCatalogItem,
    releases: toArray(resp?.releases, []),
    capabilities: toArray(resp?.capabilities, []).map((x) => String(x)),
  };
}

export function adaptCatalogReleaseListResponse(
  resp: Maybe<{ releases?: ExtensionReleaseItem[] }>,
) {
  return {
    releases: toArray(resp?.releases, []),
  };
}

export function adaptInstallationListResponse(
  resp: Maybe<{ items?: ExtensionInstallationItem[]; total?: number }>,
) {
  return {
    items: toArray(resp?.items, []),
    total: Number(resp?.total || 0),
  };
}

export function adaptInstallationDetailResponse(
  resp: Maybe<{
    installation?: ExtensionInstallationItem;
    bindings?: ExtensionBindingItem[];
    config?: Record<string, any>;
    secretRefs?: Record<string, string>;
  }>,
  fallbackItem?: ExtensionInstallationItem,
) {
  return {
    installation: (resp?.installation || fallbackItem) as ExtensionInstallationItem,
    bindings: toArray(resp?.bindings, []),
    config: (resp?.config && typeof resp.config === 'object' ? resp.config : {}) as Record<
      string,
      any
    >,
    secretRefs: (resp?.secretRefs && typeof resp.secretRefs === 'object'
      ? resp.secretRefs
      : {}) as Record<string, string>,
  };
}

export function adaptEventListResponse(
  resp: Maybe<{ items?: ExtensionEventItem[]; total?: number }>,
) {
  return {
    items: toArray(resp?.items, []),
    total: Number(resp?.total || 0),
  };
}
