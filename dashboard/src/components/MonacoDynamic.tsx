import React, { useEffect, useState } from 'react';

type EditorProps = {
  value: string;
  language?: string;
  height?: number | string;
  onChange?: (v: string) => void;
};

export const CodeEditor: React.FC<EditorProps> = ({ value, language = 'plaintext', height = 360, onChange }) => {
  const [Monaco, setMonaco] = useState<any>(null);
  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        // Try dynamic import; if not installed, fallback silently
        const mod: any = await import(/* @vite-ignore */ '@monaco-editor/react');
        if (mounted) setMonaco(mod);
      } catch (_) {
        // ignore
      }
    })();
    return () => { mounted = false; };
  }, []);
  if (!Monaco) {
    return (
      <textarea
        value={value}
        onChange={(e) => onChange && onChange(e.target.value)}
        style={{ width: '100%', height, fontFamily: 'Menlo,Consolas,monospace', fontSize: 12 }}
      />
    );
  }
  const Editor = Monaco.default || (Monaco as any).Editor || Monaco; // compat
  return (
    <Editor
      height={height}
      language={language}
      value={value}
      onChange={(v: string | undefined) => onChange && onChange(v || '')}
      options={{ minimap: { enabled: false }, wordWrap: 'on' }}
    />
  );
};

export const DiffEditor: React.FC<{ left: string; right: string; language?: string; height?: number | string }>
  = ({ left, right, language = 'plaintext', height = 420 }) => {
  const [Monaco, setMonaco] = useState<any>(null);
  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        const mod: any = await import(/* @vite-ignore */ '@monaco-editor/react');
        if (mounted) setMonaco(mod);
      } catch (_) { /* ignore */ }
    })();
    return () => { mounted = false; };
  }, []);
  if (!Monaco) {
    // fallback simple render; caller可以降级
    return null as any;
  }
  const M = Monaco.DiffEditor || (Monaco as any).default?.DiffEditor;
  if (!M) return null as any;
  return (
    <M
      height={height}
      language={language}
      original={left}
      modified={right}
      options={{ renderSideBySide: true, readOnly: true, minimap: { enabled: false }, wordWrap: 'on' }}
    />
  );
};

