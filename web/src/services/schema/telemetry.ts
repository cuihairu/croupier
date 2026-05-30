export type SchemaTelemetryEvent =
  | 'schema_load'
  | 'schema_load_error'
  | 'schema_draft_save'
  | 'schema_draft_clear'
  | 'schema_publish'
  | 'schema_publish_error';

export function trackSchemaEvent(event: SchemaTelemetryEvent, payload?: Record<string, any>) {
  if (typeof window === 'undefined') return;
  const detail = { event, payload, ts: Date.now() };
  try {
    window.dispatchEvent(new CustomEvent('croupier:schema', { detail }));
  } catch {
    // Ignore telemetry failures
  }
  if (process.env.NODE_ENV !== 'production') {
    // eslint-disable-next-line no-console
    console.info('[schema]', detail);
  }
}
