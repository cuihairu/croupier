/** MonacoDynamic：动态加载 @monaco-editor/react 的降级与装配路径——
 * 加载失败 CodeEditor 降级 textarea（受控/只读/高度/无 onChange）、DiffEditor
 * 空态；加载成功后 default/具名 Editor 回退、props 装配与 options 合并、
 * onChange 包装（undefined → ''）；DiffEditor 的 DiffEditor 导出、
 * default.DiffEditor 回退与双双缺失空态；卸载后迟到解析不触发 setMonaco。
 *
 * mock 手法：被 mock 的模块是一个可变导出的普通对象（不能是函数——源组件
 * `setMonaco(mod)` 会把函数当 useState 函数式更新调用），测试按需改写
 * default/Editor/DiffEditor 形态，或挂 then（thenable）让 await import 以
 * reject / 挂起 / 迟到 resolve。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import * as monacoModule from '@monaco-editor/react';
import { CodeEditor, DiffEditor } from './MonacoDynamic';

type MockEditorProps = Record<string, unknown>;
type MockEditor = (props: MockEditorProps) => React.ReactElement;

interface MonacoMockModule {
  __esModule?: boolean;
  default?: MockEditor | Record<string, unknown>;
  Editor?: MockEditor;
  DiffEditor?: MockEditor;
  then?: (onFulfilled: (v: unknown) => void, onRejected: (e: unknown) => void) => void;
  __make?: (testId: string) => MockEditor;
}

/** 静态 import 与组件内动态 import 共享 jest 模块注册表的同一 mock 对象，
 * 测试即可安全改写其形态。 */
const monaco = (): MonacoMockModule => monacoModule as unknown as MonacoMockModule;

/** 读取 mock 编辑器渲染时收到的 props（函数已序列化为 "[fn]"）。 */
const dumpedProps = (testId: string): Record<string, unknown> =>
  JSON.parse(screen.getByTestId(testId).getAttribute('data-props') as string) as Record<
    string,
    unknown
  >;

jest.mock('@monaco-editor/react', () => {
  const React = require('react') as typeof import('react');
  const dump = (props: MockEditorProps): string =>
    JSON.stringify(props, (_key, value) => (typeof value === 'function' ? '[fn]' : value));
  const make =
    (testId: string): MockEditor =>
    (props) =>
      React.createElement(
        'div',
        { 'data-testid': testId, 'data-props': dump(props) },
        React.createElement('button', {
          'data-testid': `${testId}-fire-change`,
          // 触发收到的 onChange（undefined 入参覆盖 v || '' 分支）
          onClick: () => {
            const cb = props.onChange;
            if (typeof cb === 'function') {
              (cb as (v?: string) => void)(undefined);
            }
          },
        }),
      );
  const mod: MonacoMockModule = {
    __esModule: true,
    default: make('monaco-default-editor'),
    Editor: make('monaco-named-editor'),
    DiffEditor: make('monaco-diff-editor'),
    __make: make,
  };
  return mod;
});

beforeEach(() => {
  const m = monaco();
  delete m.then;
  m.default = m.__make?.('monaco-default-editor');
  m.Editor = m.__make?.('monaco-named-editor');
  m.DiffEditor = m.__make?.('monaco-diff-editor');
});

/** 捕获渲染错误（模块对象非合法组件时 React 抛 invalid element type）。 */
class CaptureBoundary extends React.Component<
  { children: React.ReactNode },
  { error: Error | null }
> {
  state: { error: Error | null } = { error: null };

  static getDerivedStateFromError(error: Error): { error: Error | null } {
    return { error };
  }

  render(): React.ReactNode {
    if (this.state.error) {
      return <div data-testid="render-error">{this.state.error.message}</div>;
    }
    return this.props.children;
  }
}

describe('CodeEditor', () => {
  it('加载失败：降级 textarea，受控 onChange / readOnly / 高度生效', () => {
    monaco().then = (_onFulfilled, onRejected) => onRejected(new Error('not installed'));
    const onChange = jest.fn();
    const { container } = render(
      <CodeEditor value="原始内容" height={200} onChange={onChange} readOnly />,
    );
    const ta = container.querySelector('textarea') as HTMLTextAreaElement;
    expect(ta).toBeInTheDocument();
    expect(ta.value).toBe('原始内容');
    expect(ta.readOnly).toBe(true);
    expect(ta.style.height).toBe('200px');
    expect(ta.style.fontFamily).toContain('Menlo');

    fireEvent.change(ta, { target: { value: '改后内容' } });
    expect(onChange).toHaveBeenCalledWith('改后内容');
  });

  it('加载失败且未传 onChange：textarea 输入不报错；默认高度 360', () => {
    monaco().then = (_onFulfilled, onRejected) => onRejected(new Error('not installed'));
    const { container } = render(<CodeEditor value="x" />);
    const ta = container.querySelector('textarea') as HTMLTextAreaElement;
    expect(ta.style.height).toBe('360px');
    expect(ta.readOnly).toBe(false);
    fireEvent.change(ta, { target: { value: 'y' } });
  });

  it('加载成功：default 导出装配，全量 props 与默认 options 合并', async () => {
    const onChange = jest.fn();
    render(
      <CodeEditor
        value="let x = 1;"
        language="typescript"
        height={480}
        theme="vs-dark"
        readOnly
        onChange={onChange}
        onMount={jest.fn()}
        beforeMount={jest.fn()}
        options={{ fontSize: 12, minimap: { enabled: true } }}
      />,
    );
    const editor = await screen.findByTestId('monaco-default-editor');
    expect(editor).toBeInTheDocument();
    const props = dumpedProps('monaco-default-editor');
    expect(props.value).toBe('let x = 1;');
    expect(props.language).toBe('typescript');
    expect(props.height).toBe(480);
    expect(props.theme).toBe('vs-dark');
    expect(props.onMount).toBe('[fn]');
    expect(props.beforeMount).toBe('[fn]');
    expect(props.options).toEqual({
      minimap: { enabled: true },
      wordWrap: 'on',
      readOnly: true,
      fontSize: 12,
    });

    // mock 编辑器内部触发 onChange(undefined)：包装层兜底为 ''
    fireEvent.click(screen.getByTestId('monaco-default-editor-fire-change'));
    expect(onChange).toHaveBeenCalledWith('');
  });

  it('default 缺失：回退具名 Editor 导出；未传 options 时走 {} 合并', async () => {
    monaco().default = undefined;
    render(<CodeEditor value="v" language="json" />);
    const editor = await screen.findByTestId('monaco-named-editor');
    const props = dumpedProps('monaco-named-editor');
    expect(editor).toBeInTheDocument();
    expect(props.language).toBe('json');
    expect(props.options).toEqual({
      minimap: { enabled: false },
      wordWrap: 'on',
      readOnly: false,
    });
  });

  it('default 与 Editor 均缺失：回退模块对象自身（非组件，渲染报错被边界捕获）', async () => {
    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    const m = monaco();
    m.default = undefined;
    m.Editor = undefined;
    render(
      <CaptureBoundary>
        <CodeEditor value="fallback-via-module" />
      </CaptureBoundary>,
    );
    // 等一个宏任务确保动态 import 已 resolve、加载后分支已执行
    const caught = await screen.findByTestId('render-error', undefined, { timeout: 5000 });
    expect(caught.textContent).toMatch(/Element type is invalid/);
    errorSpy.mockRestore();
  });

  it('卸载后模块才解析：mounted 守卫拦住 setMonaco，不抛错', async () => {
    let settle: ((v: unknown) => void) | undefined;
    monaco().then = (onFulfilled) => {
      settle = onFulfilled;
    };
    const { unmount, container } = render(<CodeEditor value="pending" />);
    expect(container.querySelector('textarea')).toBeInTheDocument();
    // 先让微任务链跑到采纳 thenable（settle 已捕获、promise 仍挂起）
    await new Promise((resolve) => setTimeout(resolve, 0));
    unmount();
    // 迟到的解析（非 thenable 值，避免再次采纳 thenable）
    (settle as (v: unknown) => void)({ resolved: true });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
});

describe('DiffEditor', () => {
  it('加载失败：降级空态', async () => {
    monaco().then = (_onFulfilled, onRejected) => onRejected(new Error('not installed'));
    const { container } = render(<DiffEditor left="旧" right="新" />);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(container).toBeEmptyDOMElement();
  });

  it('加载成功：DiffEditor 导出装配，自定义 language/height 与固定 options', async () => {
    render(<DiffEditor left="旧文本" right="新文本" language="json" height={500} />);
    const editor = await screen.findByTestId('monaco-diff-editor');
    expect(editor).toBeInTheDocument();
    expect(dumpedProps('monaco-diff-editor')).toEqual({
      height: 500,
      language: 'json',
      original: '旧文本',
      modified: '新文本',
      options: {
        renderSideBySide: true,
        readOnly: true,
        minimap: { enabled: false },
        wordWrap: 'on',
      },
    });
  });

  it('默认 language=plaintext、height=420', async () => {
    render(<DiffEditor left="l" right="r" />);
    await screen.findByTestId('monaco-diff-editor');
    const props = dumpedProps('monaco-diff-editor');
    expect(props.language).toBe('plaintext');
    expect(props.height).toBe(420);
  });

  it('DiffEditor 导出缺失：回退 default.DiffEditor', async () => {
    const m = monaco();
    m.DiffEditor = undefined;
    m.default = { DiffEditor: m.__make?.('monaco-default-diff') } as Record<string, unknown>;
    render(<DiffEditor left="l" right="r" />);
    expect(await screen.findByTestId('monaco-default-diff')).toBeInTheDocument();
  });

  it('DiffEditor 与 default.DiffEditor 均缺失：渲染空态', async () => {
    const m = monaco();
    m.DiffEditor = undefined;
    m.default = undefined;
    const { container } = render(<DiffEditor left="l" right="r" />);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(container).toBeEmptyDOMElement();
  });

  it('卸载后模块才解析：mounted 守卫拦住 setMonaco，不抛错', async () => {
    let settle: ((v: unknown) => void) | undefined;
    monaco().then = (onFulfilled) => {
      settle = onFulfilled;
    };
    const { unmount } = render(<DiffEditor left="l" right="r" />);
    // 先让微任务链跑到采纳 thenable（settle 已捕获、promise 仍挂起）
    await new Promise((resolve) => setTimeout(resolve, 0));
    unmount();
    (settle as (v: unknown) => void)({ resolved: true });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
});
