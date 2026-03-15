export interface CatalogCardVM {
  extensionId: string;
  displayName: string;
  version: string;
  installed: boolean;
  tags: string[];
  defaultInstall: boolean;
}

export interface InstallationRowVM {
  installationId: number;
  extensionId: string;
  version: string;
  enabled: boolean;
  status: string;
  target: string;
}

export interface EventRowVM {
  id: number;
  type: string;
  operator: string;
  createdAt: string;
}

export function toCatalogCard(item: any): CatalogCardVM {
  return {
    extensionId: String(item.extension_id ?? ""),
    displayName: String(item.display_name ?? item.extension_id ?? ""),
    version: String(item.version ?? ""),
    installed: Boolean(item.installed),
    tags: Array.isArray(item.tags) ? item.tags.map((x: unknown) => String(x)) : [],
    defaultInstall: Boolean(item.default_install),
  };
}

export function toInstallationRow(item: any): InstallationRowVM {
  return {
    installationId: Number(item.installation_id ?? 0),
    extensionId: String(item.extension_id ?? ""),
    version: String(item.release_version ?? ""),
    enabled: Boolean(item.enabled),
    status: String(item.status ?? ""),
    target: `${String(item.target_type ?? "")}:${String(item.target_id ?? "")}`,
  };
}

export function toEventRow(item: any): EventRowVM {
  return {
    id: Number(item.id ?? 0),
    type: String(item.type ?? ""),
    operator: String(item.operator ?? ""),
    createdAt: String(item.created_at ?? ""),
  };
}
