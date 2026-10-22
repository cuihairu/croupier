/**
 * 目录管理弹窗单测（覆盖率巡检：CatalogManageModals.tsx 291 行 0% → 收口，
 * Extensions 簇余量顺序第四位；此前 Store 页套件未触达管理动作）。
 *
 * 锁定契约——CatalogRegisterModal：打开 resetFields + 预填 kind=community/
 * status=active、extensionId required 拦截、pattern 规则（transform 先 trim
 * 再校验；提交载荷为原始值，trim 由父组件承担——两侧一致的注释口径）、
 * 默认值载荷与全字段载荷（kind/status 切换）、confirmLoading 透传。
 * ReleasePublishModal：标题 displayName 兜底 name 与 item undefined 空 setTitle
 * 翼、releaseChannel 预填 stable、version required + semver pattern（含预发布
 * 后缀通过形态）、manifest 自定义校验器四翼（空/纯空白 → 必填、非 JSON →
 * 格式错、数组 → 须为对象、合法对象通过）、全字段载荷提交。
 *
 * mock 口径：@umijs/max 本地 mock；组件零服务依赖（仅 type import）。
 * Select 交互锚 .ant-modal 内按 DOM 序定位（register 弹窗 kind 先于 status）。
 *
 * 边界（诚实）：校验失败经 onOk 的 validateFields().catch 吞掉——inline 报错
 * 属正常 UX，不产生 unhandled rejection（组件注释明示，吞掉是设计行为）。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { CatalogRegisterModal, ReleasePublishModal } from '../CatalogManageModals';
import type { ExtensionCatalogItem } from '@/services/api/extensions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

const item: ExtensionCatalogItem = {
  extensionId: 'chatops',
  name: 'chatops',
  displayName: 'ChatOps',
  vendor: 'example',
  kind: 'integration',
  summary: '',
  latestVersion: '1.4.0',
  status: 'active',
  tags: [],
  installed: false,
};

/** 弹窗内第 idx 个 Select 选 option（antd6：mouseDown 落 .ant-select 根，
 * 点可见 option content 冒泡；多次开关后 DOM 里会残留 hidden 的旧 dropdown，
 * 须取当前未隐藏的那个） */
async function pickOption(modal: HTMLElement, selectIdx: number, label: string) {
  const select = modal.querySelectorAll('.ant-select')[selectIdx] as HTMLElement;
  fireEvent.mouseDown(select);
  const dropdown = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
    (d) => !d.className.includes('ant-select-dropdown-hidden'),
  ) as HTMLElement;
  expect(dropdown).not.toBeUndefined();
  fireEvent.click(
    within(dropdown).getByText(label, { selector: '.ant-select-item-option-content' }),
  );
}

/** 点弹窗 footer 主按钮（「确定」被 antd 双字插空，按选择器点） */
function clickOk() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLButtonElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/** 等校验错误文案出现（inline 报错） */
async function expectError(text: string) {
  expect(
    await screen.findByText(text, { selector: '.ant-form-item-explain-error' }),
  ).toBeInTheDocument();
}

describe('CatalogRegisterModal 登记扩展', () => {
  it('打开预填 kind=community/status=active，其余空', async () => {
    render(
      <CatalogRegisterModal open confirmLoading={false} onClose={jest.fn()} onSubmit={jest.fn()} />,
    );
    const modal = (await screen.findByText('登记扩展到目录')).closest('.ant-modal') as HTMLElement;
    expect(modal).not.toBeNull();
    // 预填显示断言锚 .ant-select 根的 textContent（antd6 选中项无稳定类名；
    // 预填语义另由载荷用例的 kind/status 默认值双保险锁定）
    const selects = modal.querySelectorAll('.ant-select');
    await waitFor(() => expect(selects[0]?.textContent).toContain('community'));
    await waitFor(() => expect(selects[1]?.textContent).toContain('active'));
    expect((screen.getByLabelText('Extension ID') as HTMLInputElement).value).toBe('');
  });

  it('extensionId required 拦截：onSubmit 不触达', async () => {
    const onSubmit = jest.fn();
    render(
      <CatalogRegisterModal open confirmLoading={false} onClose={jest.fn()} onSubmit={onSubmit} />,
    );
    await screen.findByText('登记扩展到目录');
    clickOk();
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('pattern 拦截（大写非法）→ trim 变换后合法提交：载荷为原始值（父层 trim）+ 默认 kind/status', async () => {
    const onSubmit = jest.fn();
    render(
      <CatalogRegisterModal open confirmLoading={false} onClose={jest.fn()} onSubmit={onSubmit} />,
    );
    await screen.findByText('登记扩展到目录');

    fireEvent.change(screen.getByLabelText('Extension ID'), { target: { value: 'X-Bad' } });
    clickOk();
    await expectError('小写字母或数字开头，仅含小写字母、数字、点、下划线、连字符');
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('Extension ID'), {
      target: { value: '  com.example.x  ' },
    });
    clickOk();
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    // 未触碰字段运行时为 undefined（antd store 空值），类型上的 string 由
    // 父层兜底；此处锚语义关键字段
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        extensionId: '  com.example.x  ', // transform 只作用于校验，载荷保持原值
        kind: 'community',
        status: 'active',
      }),
    );
  });

  it('全字段载荷：kind 切 ui、status 切 delisted、文本字段全填', async () => {
    const onSubmit = jest.fn();
    render(
      <CatalogRegisterModal open confirmLoading={false} onClose={jest.fn()} onSubmit={onSubmit} />,
    );
    await screen.findByText('登记扩展到目录');
    const modal = document.querySelector('.ant-modal') as HTMLElement;

    fireEvent.change(screen.getByLabelText('Extension ID'), {
      target: { value: 'com.example.demo' },
    });
    fireEvent.change(screen.getByLabelText('显示名'), { target: { value: 'Demo Ext' } });
    fireEvent.change(screen.getByLabelText('Vendor'), { target: { value: 'example' } });
    fireEvent.change(screen.getByLabelText('简介'), { target: { value: 'demo pack' } });
    fireEvent.change(screen.getByLabelText('Icon URL'), {
      target: { value: 'https://example.com/i.png' },
    });
    fireEvent.change(screen.getByLabelText('Homepage URL'), {
      target: { value: 'https://example.com' },
    });
    await pickOption(modal, 0, 'ui');
    await pickOption(modal, 1, 'delisted');

    clickOk();
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith({
      extensionId: 'com.example.demo',
      displayName: 'Demo Ext',
      vendor: 'example',
      kind: 'ui',
      summary: 'demo pack',
      iconUrl: 'https://example.com/i.png',
      homepageUrl: 'https://example.com',
      status: 'delisted',
    });
  });

  it('confirmLoading 透传：OK 按钮进入 loading', async () => {
    render(<CatalogRegisterModal open confirmLoading onClose={jest.fn()} onSubmit={jest.fn()} />);
    await screen.findByText('登记扩展到目录');
    await waitFor(() =>
      expect(
        document.querySelector('.ant-modal-footer .ant-btn-primary.ant-btn-loading'),
      ).not.toBeNull(),
    );
  });
});

describe('ReleasePublishModal 发布版本', () => {
  function renderRelease(withItem?: { item?: ExtensionCatalogItem; onSubmit?: jest.Mock }) {
    const onSubmit = withItem?.onSubmit ?? jest.fn();
    // item 显式透传（?? 默认会吞掉 undefined-item 用例）
    const itemProp = withItem && 'item' in withItem ? withItem.item : item;
    const utils = render(
      <ReleasePublishModal
        open
        item={itemProp}
        confirmLoading={false}
        onClose={jest.fn()}
        onSubmit={onSubmit}
      />,
    );
    return { ...utils, onSubmit };
  }

  it('标题 displayName；releaseChannel 预填 stable；item undefined 标题无后缀', async () => {
    const first = renderRelease();
    expect(await screen.findByText('发布版本: ChatOps')).toBeInTheDocument();
    // 预填显示锚 .ant-select 根 textContent（类名无稳定形态，载荷用例双保险）
    await waitFor(() =>
      expect(document.querySelector('.ant-modal .ant-select')?.textContent).toContain('stable'),
    );
    first.unmount();

    renderRelease({ item: undefined });
    expect(await screen.findByText('发布版本')).toBeInTheDocument();
  });

  it('标题兜底翼：displayName 空 → name', async () => {
    renderRelease({ item: { ...item, displayName: '' } });
    expect(await screen.findByText('发布版本: chatops')).toBeInTheDocument();
  });

  it('version required/pattern 拦截；semver 预发布后缀通过', async () => {
    const onSubmit = jest.fn();
    renderRelease({ onSubmit });
    await screen.findByText('发布版本: ChatOps');

    clickOk();
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('版本号'), { target: { value: '1.2' } });
    clickOk();
    await expectError('须为 semver 形态，如 1.2.3 或 1.0.0-beta.1');
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('版本号'), {
      target: { value: '1.0.0-beta.1+build' },
    });
    fireEvent.change(screen.getByLabelText('Manifest (JSON)'), {
      target: { value: '{"capabilities":[]}' },
    });
    clickOk();
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ version: '1.0.0-beta.1+build' }),
    );
  });

  it('manifest 校验器四翼：空/纯空白 → 必填；非 JSON → 格式错；数组 → 须为对象', async () => {
    const onSubmit = jest.fn();
    renderRelease({ onSubmit });
    await screen.findByText('发布版本: ChatOps');
    fireEvent.change(screen.getByLabelText('版本号'), { target: { value: '1.2.3' } });

    const manifest = screen.getByLabelText('Manifest (JSON)');

    clickOk();
    await expectError('manifest 必填（发布版本须携带能力/页面清单）');
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.change(manifest, { target: { value: '   ' } });
    clickOk();
    await expectError('manifest 必填（发布版本须携带能力/页面清单）');

    fireEvent.change(manifest, { target: { value: 'nope' } });
    clickOk();
    await expectError('Manifest JSON 格式不正确');

    fireEvent.change(manifest, { target: { value: '[1,2]' } });
    clickOk();
    await expectError('manifest 须为 JSON 对象');
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('全字段载荷：channel 切 beta + 文本字段全填', async () => {
    const onSubmit = jest.fn();
    renderRelease({ onSubmit });
    await screen.findByText('发布版本: ChatOps');
    const modal = document.querySelector('.ant-modal') as HTMLElement;

    fireEvent.change(screen.getByLabelText('版本号'), { target: { value: '2.0.0' } });
    await pickOption(modal, 0, 'beta');
    fireEvent.change(screen.getByLabelText('最低核心版本'), { target: { value: '0.0.1' } });
    fireEvent.change(screen.getByLabelText('包引用'), {
      target: { value: 'packs/com.example-2.0.0.tgz' },
    });
    fireEvent.change(screen.getByLabelText('校验和'), {
      target: { value: 'sha256:abc' },
    });
    fireEvent.change(screen.getByLabelText('Changelog'), { target: { value: 'second' } });
    fireEvent.change(screen.getByLabelText('Manifest (JSON)'), {
      target: { value: '{"capabilities":["x"]}' },
    });

    clickOk();
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith({
      version: '2.0.0',
      releaseChannel: 'beta',
      minCoreVersion: '0.0.1',
      packageRef: 'packs/com.example-2.0.0.tgz',
      checksum: 'sha256:abc',
      changelog: 'second',
      manifestJson: '{"capabilities":["x"]}',
    });
  });
});
