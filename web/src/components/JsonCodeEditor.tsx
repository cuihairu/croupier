import React from 'react';
import { CodeEditor } from '@/components/MonacoDynamic';

/** Sublime（Monokai）配色的共享主题名——CodeEditor 经 beforeMount 注册。 */
export const SUBLIME_THEME = 'croupier-sublime';

interface MonacoEditorNs {
  editor: {
    defineTheme: (name: string, data: Record<string, unknown>) => void;
  };
}

let themeRegistered = false;

export function defineSublimeTheme(monaco: unknown): void {
  if (themeRegistered || !monaco) return;
  const m = monaco as MonacoEditorNs;
  m.editor.defineTheme(SUBLIME_THEME, {
    base: 'vs-dark',
    inherit: true,
    rules: [
      { token: 'string', foreground: 'E6DB74' },
      { token: 'number', foreground: 'AE81FF' },
      { token: 'constant', foreground: 'AE81FF' },
      { token: 'keyword', foreground: 'F92672' },
      { token: 'comment', foreground: '75715E', fontStyle: 'italic' },
      { token: 'delimiter', foreground: 'F8F8F2' },
    ],
    colors: {
      'editor.background': '#272822',
      'editor.foreground': '#F8F8F2',
      'editorLineNumber.foreground': '#75715E',
      'editorLineNumber.activeForeground': '#C2C2BF',
      'editorGutter.background': '#272822',
      'editor.lineHighlightBackground': '#3E3D32',
      'editor.selectionBackground': '#49483E',
      'editorCursor.foreground': '#F8F8F0',
      'editorIndentGuide.background1': '#3B3A32',
    },
  });
  themeRegistered = true;
}

/** 参数/JSON 编辑统一入口：行号常显、Sublime(Monokai) 深色风格。
 * monaco 加载失败时 CodeEditor 内部降级为 textarea（行为不变）。 */
export default function JsonCodeEditor({
  value,
  onChange,
  height = 260,
  readOnly,
}: {
  value: string;
  onChange: (v: string) => void;
  height?: number | string;
  readOnly?: boolean;
}) {
  return (
    <CodeEditor
      value={value}
      onChange={onChange}
      language="json"
      height={height}
      readOnly={readOnly}
      theme={SUBLIME_THEME}
      beforeMount={defineSublimeTheme}
      options={{
        lineNumbers: 'on',
        folding: true,
        tabSize: 2,
        fontSize: 13,
        fontFamily: 'Menlo, Consolas, "Courier New", monospace',
        formatOnPaste: true,
        scrollBeyondLastLine: false,
        renderLineHighlight: 'all',
      }}
    />
  );
}
