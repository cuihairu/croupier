/**
 * AvatarModal 本体（R55 覆盖批次）：提交/上传流。
 *
 * 宿主 Profile/index.test.tsx 只测 hero「更换头像」入口开合（其注释注明
 * 「提交/上传流属各组件自有套件域」）。本套件补齐行为面：
 * 回填初始值、URL 合法/空值/非法串提交分支、持久化失败不关窗、
 * 拖拽上传成功自动持久化、上传失败只报错。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import AvatarModal from '../AvatarModal';
import { updateMyProfile } from '@/services/api/me';
import { buildAvatarObjectKey, uploadAsset } from '@/services/api/storage';

jest.mock('@/services/api/me', () => ({ updateMyProfile: jest.fn() }));
jest.mock('@/services/api/storage', () => ({
  uploadAsset: jest.fn(),
  buildAvatarObjectKey: jest.fn(() => 'avatars/x.png'),
}));

// 工厂自包含：页面 formatMessage 全部 id-only（无 defaultMessage 兜底）——
// 必须用真实 zh-CN 词表查表（同 Profile/index.test.tsx 先例），否则全局
// mock 只回 defaultMessage 得到 undefined，placeholder/标题全空。
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
    useModel: () => ({ initialState: {}, loading: false, refresh: jest.fn() }),
  };
});

const mockUpdate = updateMyProfile as jest.MockedFunction<typeof updateMyProfile>;
const mockUpload = uploadAsset as jest.MockedFunction<typeof uploadAsset>;

type Harness = {
  onClose: jest.Mock;
  onPersisted: jest.Mock;
};

function renderModal(props: Partial<React.ComponentProps<typeof AvatarModal>> = {}): Harness {
  const harness: Harness = {
    onClose: jest.fn(() => Promise.resolve()),
    onPersisted: jest.fn(() => Promise.resolve()),
  };
  render(
    <App>
      <AvatarModal
        open
        avatar=""
        displayName="系统管理员"
        username="admin"
        onClose={harness.onClose}
        onPersisted={harness.onPersisted}
        {...props}
      />
    </App>,
  );
  return harness;
}

const avatarInput = async () => {
  await screen.findByPlaceholderText('https://example.com/avatar.png');
  return screen.getByPlaceholderText('https://example.com/avatar.png') as HTMLInputElement;
};

const submitButton = async () => screen.findByRole('button', { name: /保存头像/ });

beforeEach(() => {
  jest.clearAllMocks();
  mockUpdate.mockResolvedValue({} as Awaited<ReturnType<typeof updateMyProfile>>);
});

describe('AvatarModal URL 提交流', () => {
  it('open 时回填当前头像到 URL 输入框并渲染预览', async () => {
    renderModal({ avatar: 'https://cdn.example.com/old.png' });
    const input = await avatarInput();
    expect(input.value).toBe('https://cdn.example.com/old.png');
    expect(document.querySelector('.avatar-upload-preview')).toBeInTheDocument();
  });

  it('合法 URL 提交 → 持久化 + 关窗 + 刷新', async () => {
    const { onClose, onPersisted } = renderModal();
    const input = await avatarInput();
    fireEvent.change(input, { target: { value: 'https://cdn.example.com/new.png' } });
    fireEvent.click(await submitButton());
    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith({ avatar: 'https://cdn.example.com/new.png' }),
    );
    expect(await screen.findByText('头像更新成功')).toBeInTheDocument();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onPersisted).toHaveBeenCalled();
  });

  it('空值提交被 required 拦截，不发请求', async () => {
    const { onClose } = renderModal();
    fireEvent.click(await submitButton());
    // required 与自定义 validator 对空值给出同文案，两个 explain 节点都在
    expect((await screen.findAllByText('请输入头像 URL')).length).toBeGreaterThan(0);
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('非法 URL 被 validator 拦截，不发请求', async () => {
    const { onClose } = renderModal();
    const input = await avatarInput();
    fireEvent.change(input, { target: { value: 'not-a-url' } });
    fireEvent.click(await submitButton());
    expect(await screen.findByText('请输入合法 URL')).toBeInTheDocument();
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('持久化失败报错且不关窗不刷新', async () => {
    mockUpdate.mockRejectedValue(new Error('boom'));
    const { onClose, onPersisted } = renderModal();
    const input = await avatarInput();
    fireEvent.change(input, { target: { value: 'https://cdn.example.com/x.png' } });
    fireEvent.click(await submitButton());
    expect(await screen.findByText('更新个人信息失败')).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(onPersisted).not.toHaveBeenCalled();
  });

  it('关闭按钮触发 onClose', async () => {
    const { onClose } = renderModal();
    const cancel = await screen.findByRole('button', { name: /取\s*消/ });
    fireEvent.click(cancel);
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});

describe('AvatarModal 拖拽上传流', () => {
  const fireFile = (file: File) => {
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file] } });
  };

  it('上传成功 → 自动回填 URL 并持久化 + 关窗', async () => {
    mockUpload.mockResolvedValue({ URL: 'https://cdn.example.com/up.png' } as Awaited<
      ReturnType<typeof uploadAsset>
    >);
    const { onClose, onPersisted } = renderModal();
    await screen.findByPlaceholderText('https://example.com/avatar.png');
    fireFile(new File(['x'], 'me.png', { type: 'image/png' }));
    await waitFor(() =>
      expect(mockUpload).toHaveBeenCalledWith(
        expect.any(File),
        expect.objectContaining({ path: 'avatars/x.png' }),
      ),
    );
    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith({ avatar: 'https://cdn.example.com/up.png' }),
    );
    expect(await screen.findByText('头像更新成功')).toBeInTheDocument();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onPersisted).toHaveBeenCalled();
  });

  it('上传返回缺失 URL 当失败处理，只报错不持久化', async () => {
    mockUpload.mockResolvedValue({} as Awaited<ReturnType<typeof uploadAsset>>);
    const { onClose } = renderModal();
    await screen.findByPlaceholderText('https://example.com/avatar.png');
    fireFile(new File(['x'], 'me.png', { type: 'image/png' }));
    expect(await screen.findByText('更新个人信息失败')).toBeInTheDocument();
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('上传接口抛错只报错不持久化', async () => {
    mockUpload.mockRejectedValue(new Error('net down'));
    const { onClose } = renderModal();
    await screen.findByPlaceholderText('https://example.com/avatar.png');
    fireFile(new File(['x'], 'me.png', { type: 'image/png' }));
    expect(await screen.findByText('更新个人信息失败')).toBeInTheDocument();
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('上传接口抛非 Error 值同样收口（onError 归一为 Error）', async () => {
    mockUpload.mockRejectedValue('plain-string-reject');
    const { onClose } = renderModal();
    await screen.findByPlaceholderText('https://example.com/avatar.png');
    fireFile(new File(['x'], 'me.png', { type: 'image/png' }));
    expect(await screen.findByText('更新个人信息失败')).toBeInTheDocument();
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
