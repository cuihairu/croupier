export type WorkspaceTelemetryEvent =
  | 'workspace_page_open'
  | 'workspace_load'
  | 'workspace_load_error'
  | 'workspace_render_error'
  | 'workspace_create_entry'
  | 'workspace_clone'
  | 'workspace_save'
  | 'workspace_save_error'
  | 'workspace_publish'
  | 'workspace_publish_error'
  | 'workspace_unpublish'
  | 'workspace_unpublish_error'
  | 'workspace_delete'
  | 'workspace_delete_error'
  | 'workspace_template_apply'
  | 'workspace_template_apply_error'
  | 'workspace_versions_load'
  | 'workspace_versions_load_error'
  | 'workspace_rollback'
  | 'workspace_rollback_error'
  | 'workspace_import'
  | 'workspace_import_error'
  | 'workspace_backup_export'
  | 'workspace_backup_export_error';

type WorkspaceTelemetryListener = (
  event: WorkspaceTelemetryEvent,
  payload?: Record<string, any>,
) => void;

const listeners = new Set<WorkspaceTelemetryListener>();

export function subscribeWorkspaceTelemetry(listener: WorkspaceTelemetryListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function trackWorkspaceEvent(event: WorkspaceTelemetryEvent, payload?: Record<string, any>) {
  listeners.forEach((listener) => {
    try {
      listener(event, payload);
    } catch {
      // Ignore listener failures
    }
  });

  if (typeof window === 'undefined') return;
  const detail = { event, payload, ts: Date.now() };
  try {
    const Ctor = (window as any).CustomEvent;
    if (typeof Ctor === 'function') {
      window.dispatchEvent(new Ctor('croupier:workspace', { detail }));
    } else {
      const fallbackEvent = document.createEvent('CustomEvent');
      fallbackEvent.initCustomEvent('croupier:workspace', false, false, detail);
      window.dispatchEvent(fallbackEvent);
    }
  } catch {
    try {
      const fallbackEvent = document.createEvent('CustomEvent');
      fallbackEvent.initCustomEvent('croupier:workspace', false, false, detail);
      window.dispatchEvent(fallbackEvent);
    } catch {
      // Ignore telemetry failures
    }
  }
  if (process.env.NODE_ENV !== 'production') {
    console.info('[workspace]', detail);
  }
}
