/**
 * GameIconField（游戏图标字段）：URL 手填 / 文件上传双方式的行为面。
 *
 * 上传走 antd Upload 的 input[type=file] 注入（同 AvatarModal 套件先例），
 * 服务层 uploadGameIcon 整体 mock；客户端预校验为导出的纯函数直测。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, Form } from 'antd';
import GameIconField, { validateGameIconFile } from '../index';
import { GAME_ICON_MAX_BYTES, uploadGameIcon } from '@/services/api/games';

jest.mock('@/services/api/games', () => ({
  uploadGameIcon: jest.fn(),
  GAME_ICON_MAX_BYTES: 2 * 1024 * 1024,
}));

// 同 AvatarModal 套件工厂：pages 词表查表 + defaultMessage 兜底；本组件的
// components.gameIconField.* 词不进词表，回落 defaultMessage 即可。
jest.mock('@umijs/max', () => {
  const pagesLocale = require('@/locales/zh-CN/pages').default as Record<string, string>;
  const fmt = (
    opts: { id: string; defaultMessage?: string },
    values?: Record<string, unknown>,
  ): string => {
    let msg = pagesLocale[opts.id] ?? opts.defaultMessage ?? opts.id;
    if (values) {
      for (const [k, v] of Object.entries(values)) msg = msg.split(`{${k}}`).join(String(v));
    }
    return msg;
  };
  const intl = { formatMessage: fmt, locale: 'zh-CN' };
  return {
    FormattedMessage: (props: { id: string; defaultMessage?: string }) => (
      <>{fmt({ id: props.id, defaultMessage: props.defaultMessage })}</>
    ),
    useIntl: () => intl,
    getIntl: () => intl,
  };
});

const mockUpload = uploadGameIcon as jest.MockedFunction<typeof uploadGameIcon>;

type Harness = { onChange: jest.Mock };

function renderField(props: Partial<React.ComponentProps<typeof GameIconField>> = {}): Harness {
  const harness: Harness = { onChange: jest.fn() };
  render(
    <App>
      <GameIconField
        value="https://cdn.example.com/old.png"
        onChange={harness.onChange}
        {...props}
      />
    </App>,
  );
  return harness;
}

const iconInput = async () => {
  const input = (await screen.findByTestId('game-icon-input')) as HTMLInputElement;
  await waitFor(() => expect(input.value).toBe('https://cdn.example.com/old.png'));
  return input;
};

const fireFile = (file: File) => {
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  fireEvent.change(input, { target: { files: [file] } });
};

const fmt = ({ defaultMessage }: { id?: string; defaultMessage: string }) => defaultMessage;

describe('validateGameIconFile（客户端预校验纯函数）', () => {
  it('按 MIME 或 .svg 扩展名放行白名单类型', () => {
    expect(validateGameIconFile(new File(['x'], 'a.png', { type: 'image/png' }), fmt)).toBeNull();
    expect(validateGameIconFile(new File(['x'], 'b.webp', { type: 'image/webp' }), fmt)).toBeNull();
    // MIME 缺失时按扩展名兜底（svg 常被浏览器标空或 text/plain）
    expect(validateGameIconFile(new File(['x'], 'c.svg', { type: '' }), fmt)).toBeNull();
  });

  it('拒绝白名单外类型与超过 2MB 的文件', () => {
    expect(validateGameIconFile(new File(['x'], 'a.txt', { type: 'text/plain' }), fmt)).toContain(
      '仅支持',
    );
    const big = new File(['x'.repeat(GAME_ICON_MAX_BYTES + 1)], 'big.png', {
      type: 'image/png',
    });
    expect(validateGameIconFile(big, fmt)).toContain('2MB');
    // 恰好到上限放行
    const exact = new File(['x'.repeat(GAME_ICON_MAX_BYTES)], 'exact.png', { type: 'image/png' });
    expect(validateGameIconFile(exact, fmt)).toBeNull();
  });
});

describe('GameIconField 行为面', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('渲染 URL 输入框（含 GameIcon 预览前缀）与上传按钮', async () => {
    renderField();
    const input = await iconInput();
    expect(input.value).toBe('https://cdn.example.com/old.png');
    expect(screen.getByRole('button', { name: /上传图标/ })).toBeInTheDocument();
  });

  it('URL 手填直接走受控 onChange', async () => {
    const { onChange } = renderField();
    const input = await iconInput();
    fireEvent.change(input, { target: { value: 'https://cdn.example.com/new.png' } });
    expect(onChange).toHaveBeenCalledWith('https://cdn.example.com/new.png');
  });

  it('上传成功：服务返回 url 自动回填 onChange + 成功提示', async () => {
    mockUpload.mockResolvedValue({
      key: 'icons/games/4d5e562f37cd6576.png',
      url: '/uploads/icons/games/4d5e562f37cd6576.png',
    });
    const { onChange } = renderField();
    await iconInput();
    fireFile(new File(['x'], 'logo.png', { type: 'image/png' }));
    await waitFor(() =>
      expect(mockUpload).toHaveBeenCalledWith(expect.any(File), expect.anything()),
    );
    await waitFor(() =>
      expect(onChange).toHaveBeenCalledWith('/uploads/icons/games/4d5e562f37cd6576.png'),
    );
    expect(await screen.findByText('图标已上传并回填')).toBeInTheDocument();
  });

  it('上传中展示进度条，结束后消失', async () => {
    let resolveUpload: (value: { key: string; url: string }) => void = () => {};
    mockUpload.mockReturnValue(
      new Promise((resolve) => {
        resolveUpload = resolve;
      }),
    );
    const { onChange } = renderField();
    await iconInput();
    fireFile(new File(['x'], 'logo.png', { type: 'image/png' }));
    await screen.findByTestId('game-icon-progress');
    resolveUpload({ key: 'k', url: '/uploads/icons/games/ok.png' });
    await waitFor(() => expect(onChange).toHaveBeenCalledWith('/uploads/icons/games/ok.png'));
    await waitFor(() => expect(screen.queryByTestId('game-icon-progress')).not.toBeInTheDocument());
  });

  it('上传失败：透传服务层错误文案且不改表单值', async () => {
    mockUpload.mockRejectedValue(new Error('仅支持 png/jpg/webp/svg 格式的图标'));
    const { onChange } = renderField();
    await iconInput();
    fireFile(new File(['x'], 'logo.png', { type: 'image/png' }));
    expect(await screen.findByText('仅支持 png/jpg/webp/svg 格式的图标')).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });

  it('类型不在白名单：客户端直接拒绝，不发起上传请求', async () => {
    renderField();
    await iconInput();
    fireFile(new File(['x'], 'notes.txt', { type: 'text/plain' }));
    expect(await screen.findByText('仅支持 png/jpg/webp/svg 格式的图标')).toBeInTheDocument();
    expect(mockUpload).not.toHaveBeenCalled();
  });

  it('超过大小上限：客户端直接拒绝，不发起上传请求', async () => {
    renderField();
    await iconInput();
    fireFile(new File(['x'.repeat(GAME_ICON_MAX_BYTES + 1)], 'big.png', { type: 'image/png' }));
    expect(await screen.findByText('图标文件超过 2MB 大小上限')).toBeInTheDocument();
    expect(mockUpload).not.toHaveBeenCalled();
  });
});
