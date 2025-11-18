// Renderer registry for GM functions views
import React from 'react';

// Renderer function type
type Renderer = (props: { data: any; options?: any }) => React.ReactNode;

// Registry of renderers
const renderers: Record<string, Renderer> = {};

// Default JSON view renderer
const JsonView: Renderer = ({ data, options }) => {
  return <pre style={{ whiteSpace: 'pre-wrap' }}>{typeof data === 'string' ? data : JSON.stringify(data, null, 2)}</pre>;
};

// Text view renderer
const TextView: Renderer = ({ data }) => {
  return <div>{String(data ?? '')}</div>;
};

// Number view renderer
const NumberView: Renderer = ({ data }) => {
  return <div>{Number(data ?? 0)}</div>;
};

// Table view renderer (for arrays of objects)
const TableView: Renderer = ({ data }) => {
  if (!Array.isArray(data) || data.length === 0) {
    return <div>No data</div>;
  }
  
  const headers = Object.keys(data[0]);
  return (
    <table style={{ borderCollapse: 'collapse', width: '100%' }}>
      <thead>
        <tr>
          {headers.map(header => (
            <th key={header} style={{ border: '1px solid #ddd', padding: '8px', textAlign: 'left' }}>
              {header}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {data.map((row, index) => (
          <tr key={index}>
            {headers.map(header => (
              <td key={header} style={{ border: '1px solid #ddd', padding: '8px' }}>
                {String(row[header] ?? '')}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
};

/**
 * Get a renderer by name
 * @param name Renderer name (e.g., 'json.view', 'text.view', 'table.view')
 * @returns Renderer function or null if not found
 */
export function getRenderer(name: string): Renderer | null {
  return renderers[name] || null;
}

/**
 * Register built-in renderers
 */
export function registerBuiltins(): void {
  // Register default renderers
  renderers['json.view'] = JsonView;
  renderers['text.view'] = TextView;
  renderers['number.view'] = NumberView;
  renderers['table.view'] = TableView;
  
  // Aliases for backward compatibility
  renderers['json'] = JsonView;
  renderers['text'] = TextView;
  renderers['number'] = NumberView;
  renderers['table'] = TableView;
}

/**
 * Load pack plugins (placeholder for future extension)
 * This would typically load renderers from external packs
 */
export async function loadPackPlugins(): Promise<void> {
  // This is a placeholder for loading external pack plugins
  // In a real implementation, this would:
  // 1. Fetch available packs from an API
  // 2. Load pack-specific renderers
  // 3. Register them in the renderers registry
  
  console.log('Pack plugins loading placeholder - no external packs configured');
  
  // Simulate async loading
  await new Promise(resolve => setTimeout(resolve, 100));
}

/**
 * Register a custom renderer
 * @param name Renderer name
 * @param renderer Renderer function
 */
export function registerRenderer(name: string, renderer: Renderer): void {
  renderers[name] = renderer;
}

/**
 * Unregister a renderer
 * @param name Renderer name
 */
export function unregisterRenderer(name: string): void {
  delete renderers[name];
}

/**
 * Get list of available renderer names
 */
export function getAvailableRenderers(): string[] {
  return Object.keys(renderers);
}

// Auto-register builtins when module loads
registerBuiltins();