// Template wrapper around generated contracts/client.
// Adjust imports to match your dashboard build setup.

export interface PaginationQuery {
  page?: number;
  page_size?: number;
}

export interface ExtensionApi {
  listCatalog(query?: PaginationQuery): Promise<any>;
  listInstallations(query?: PaginationQuery): Promise<any>;
  install(payload: Record<string, unknown>): Promise<any>;
  enable(id: string): Promise<any>;
  disable(id: string): Promise<any>;
  upgrade(id: string, payload: Record<string, unknown>): Promise<any>;
  uninstall(id: string): Promise<any>;
  listEvents(id: string, query?: PaginationQuery): Promise<any>;
}

export function createExtensionApi(http: {
  get(url: string, config?: { params?: Record<string, unknown> }): Promise<any>;
  post(url: string, body?: Record<string, unknown>): Promise<any>;
  delete(url: string): Promise<any>;
}): ExtensionApi {
  return {
    listCatalog(query) {
      return http.get("/api/v1/extensions/catalog", { params: query });
    },
    listInstallations(query) {
      return http.get("/api/v1/extensions/installations", { params: query });
    },
    install(payload) {
      return http.post("/api/v1/extensions/install", payload);
    },
    enable(id) {
      return http.post(`/api/v1/extensions/${id}/enable`);
    },
    disable(id) {
      return http.post(`/api/v1/extensions/${id}/disable`);
    },
    upgrade(id, payload) {
      return http.post(`/api/v1/extensions/${id}/upgrade`, payload);
    },
    uninstall(id) {
      return http.delete(`/api/v1/extensions/${id}/uninstall`);
    },
    listEvents(id, query) {
      return http.get(`/api/v1/extensions/${id}/events`, { params: query });
    },
  };
}
