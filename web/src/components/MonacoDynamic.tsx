import React, { useEffect, useState } from 'react';

type EditorProps = {
  value: string;
  language?: string;
  height?: number | string;
  onChange?: (v: string) => void;
  onMount?: (editor: unknown, monaco: unknown) => void;
  readOnly?: boolean;
  theme?: string;
  options?: Record<string, unknown>;
  beforeMount?: (monaco: unknown) => void;
};

export const CodeEditor: React.FC<EditorProps> = ({
  value,
  language = 'plaintext',
  height = 360,
  onChange,
  onMount,
  readOnly,
  theme,
  options,
  beforeMount,
}) => {
  const [Monaco, setMonaco] = useState<unknown>(null);
  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        // Try dynamic import; if not installed, fallback silently
        const mod = await import('@monaco-editor/react');
        if (mounted) setMonaco(mod);
      } catch {
        // ignore
      }
    })();
    return () => {
      mounted = false;
    };
  }, []);
  if (!Monaco) {
    return (
      <textarea
        value={value}
        onChange={(e) => onChange && onChange(e.target.value)}
        readOnly={!!readOnly}
        style={{ width: '100%', height, fontFamily: 'Menlo,Consolas,monospace', fontSize: 12 }}
      />
    );
  }
  const mod = Monaco as Record<string, unknown>;
  const Editor = (mod.default || mod.Editor || Monaco) as React.ComponentType<
    Record<string, unknown>
  >;
  return (
    <Editor
      height={height}
      language={language}
      value={value}
      theme={theme}
      beforeMount={beforeMount}
      onChange={(v: string | undefined) => onChange && onChange(v || '')}
      onMount={onMount}
      options={{
        minimap: { enabled: false },
        wordWrap: 'on',
        readOnly: !!readOnly,
        ...(options || {}),
      }}
    />
  );
};

export const DiffEditor: React.FC<{
  left: string;
  right: string;
  language?: string;
  height?: number | string;
}> = ({ left, right, language = 'plaintext', height = 420 }) => {
  const [Monaco, setMonaco] = useState<unknown>(null);
  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        const mod = await import('@monaco-editor/react');
        if (mounted) setMonaco(mod);
      } catch {
        /* ignore */
      }
    })();
    return () => {
      mounted = false;
    };
  }, []);
  if (!Monaco) {
    // fallback simple render; caller可以降级
    return null;
  }
  const mod = Monaco as Record<string, unknown>;
  const defaultMod = mod.default as Record<string, unknown> | undefined;
  const M = (mod.DiffEditor || defaultMod?.DiffEditor) as
    | React.ComponentType<Record<string, unknown>>
    | undefined;
  if (!M) return null;
  return (
    <M
      height={height}
      language={language}
      original={left}
      modified={right}
      options={{
        renderSideBySide: true,
        readOnly: true,
        minimap: { enabled: false },
        wordWrap: 'on',
      }}
    />
  );
};
