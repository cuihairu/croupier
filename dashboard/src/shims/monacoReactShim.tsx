// Lightweight shim for '@monaco-editor/react' to keep build working when the
// dependency is not installed or is intentionally disabled. It provides
// minimal Editor/DiffEditor components so pages can gracefully degrade.
//
// To use the real Monaco editor:
//   1) install deps: `pnpm add @monaco-editor/react monaco-editor`
//   2) set env `USE_MONACO=1` when running dev/build to bypass the alias in
//      `web/config/config.ts`.

import React from 'react';

type EditorProps = {
  value?: string;
  language?: string;
  height?: number | string;
  onChange?: (v: string | undefined) => void;
  options?: any;
};

export const Editor: React.FC<EditorProps> = ({ value = '', height = 360, onChange }) => {
  return (
    <textarea
      value={value}
      onChange={(e) => onChange?.(e.target.value)}
      style={{ width: '100%', height, fontFamily: 'Menlo,Consolas,monospace', fontSize: 12 }}
    />
  );
};

export const DiffEditor: React.FC<{
  original?: string;
  modified?: string;
  language?: string;
  height?: number | string;
  options?: any;
}> = ({ original = '', modified = '', height = 420 }) => {
  return (
    <div style={{ display: 'flex', gap: 8 }}>
      <textarea
        readOnly
        value={original}
        style={{ width: '50%', height, fontFamily: 'Menlo,Consolas,monospace', fontSize: 12, background: '#fafafa' }}
      />
      <textarea
        readOnly
        value={modified}
        style={{ width: '50%', height, fontFamily: 'Menlo,Consolas,monospace', fontSize: 12, background: '#fff' }}
      />
    </div>
  );
};

export default { Editor, DiffEditor };

