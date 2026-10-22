/**
 * JSON 编辑器装配层单测（覆盖率补缺轮：components/JsonCodeEditor.tsx 78 行
 * 0% → 收口——消费方 Invoke/RequestBodyEditor 与 Instances/DebugModal 的套件
 * 未触达本组件，v8 基线 0/0/0/0 实证）。
 *
 * 锁定契约：
 * - SUBLIME_THEME 常量 = 'croupier-sublime'（对外契约键，主题注册与
 *   CodeEditor theme 透传同源）；
 * - defineSublimeTheme 守卫链：monaco 为 null/undefined 直接返回不抛；
 *   首次调用 defineTheme 注册完整主题负载（rules 六 token + colors 十色键）；
 *   模块级 themeRegistered 记忆化——二次调用 no-op（只注册一次）；
 * - 默认导出 = CodeEditor 配置包装：value/onChange/height(默认 260)/readOnly
 *   透传、language='json'、theme=SUBLIME_THEME、beforeMount 绑定
 *   defineSublimeTheme（引用同一函数）、options 八键（lineNumbers on /
 *   folding / tabSize 2 / fontSize 13 / formatOnPaste / scrollBeyondLastLine /
 *   renderLineHighlight all / 等宽字体族）。
 *
 * mock 口径：@/components/MonacoDynamic 的 CodeEditor mock 成捕获 props 的
 * 组件桩（monaco 在 jsdom 不可渲染，配置契约即本组件全部逻辑；主题注册经
 * defineSublimeTheme 直测覆盖）。坑实证 R26：jest.fn 在工厂内、测试侧经
 * mock 后模块导出取句柄。themeRegistered 为模块级单例——注册序列集中在
 * 同一用例内按序断言，防跨用例顺序依赖。
 */
import React from 'react';
import { render } from '@testing-library/react';
import JsonCodeEditor, { SUBLIME_THEME, defineSublimeTheme } from './JsonCodeEditor';
import { CodeEditor } from '@/components/MonacoDynamic';

jest.mock('@/components/MonacoDynamic', () => ({
  CodeEditor: jest.fn(() => null),
}));

const mCode = CodeEditor as unknown as jest.Mock;

/** 记录 defineTheme 调用的 monaco 桩 */
function fakeMonaco() {
  return { editor: { defineTheme: jest.fn() } };
}

describe('SUBLIME_THEME 与主题注册', () => {
  it('常量契约 + 守卫链：null 不抛 → 首次注册完整负载 → 二次 no-op', () => {
    expect(SUBLIME_THEME).toBe('croupier-sublime');

    // 翼一：!monaco 守卫（不抛、不调 defineTheme）
    expect(() => defineSublimeTheme(null)).not.toThrow();
    expect(() => defineSublimeTheme(undefined)).not.toThrow();

    // 首次真实注册：完整主题负载
    const first = fakeMonaco();
    defineSublimeTheme(first);
    expect(first.editor.defineTheme).toHaveBeenCalledTimes(1);
    const [name, data] = first.editor.defineTheme.mock.calls[0] as [
      string,
      {
        base: string;
        inherit: boolean;
        rules: { token: string }[];
        colors: Record<string, string>;
      },
    ];
    expect(name).toBe('croupier-sublime');
    expect(data.base).toBe('vs-dark');
    expect(data.inherit).toBe(true);
    // 六类 token 规则（string/number/constant/keyword/comment/delimiter）
    expect(data.rules.map((r) => r.token)).toEqual([
      'string',
      'number',
      'constant',
      'keyword',
      'comment',
      'delimiter',
    ]);
    expect(data.colors['editor.background']).toBe('#272822');
    expect(data.colors['editorCursor.foreground']).toBe('#F8F8F0');

    // 记忆化：二次调用（含新 monaco 实例）no-op
    const second = fakeMonaco();
    defineSublimeTheme(second);
    expect(second.editor.defineTheme).not.toHaveBeenCalled();
  });
});

describe('JsonCodeEditor 装配', () => {
  it('默认包装：height 260、theme、json、options 全键透传', () => {
    mCode.mockClear();
    const onChange = jest.fn();
    render(<JsonCodeEditor value={'{"a":1}'} onChange={onChange} />);

    expect(mCode).toHaveBeenCalledTimes(1);
    const props = mCode.mock.calls[0][0] as Record<string, unknown>;
    expect(props.value).toBe('{"a":1}');
    expect(props.onChange).toBe(onChange);
    expect(props.language).toBe('json');
    expect(props.height).toBe(260); // 默认高
    expect(props.readOnly).toBeUndefined(); // 未传
    expect(props.theme).toBe(SUBLIME_THEME);
    expect(props.beforeMount).toBe(defineSublimeTheme); // 绑定同一引用
    expect(props.options).toEqual({
      lineNumbers: 'on',
      folding: true,
      tabSize: 2,
      fontSize: 13,
      fontFamily: 'Menlo, Consolas, "Courier New", monospace',
      formatOnPaste: true,
      scrollBeyondLastLine: false,
      renderLineHighlight: 'all',
    });
  });

  it('显式 height/readOnly 透传', () => {
    mCode.mockClear();
    render(<JsonCodeEditor value="x" onChange={jest.fn()} height={480} readOnly />);

    const props = mCode.mock.calls[0][0] as Record<string, unknown>;
    expect(props.height).toBe(480);
    expect(props.readOnly).toBe(true);
  });
});
