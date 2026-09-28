/**
 * 安装详情抽屉单测（覆盖率巡检：Extensions 簇 0% → 行覆盖收口）。
 *
 * 锁定契约：打开加载链（detail → adapter 兜底 → schema/config 各自失败静默
 * 兜底、配置/密文回显）、概览四项（启用态/健康/版本/绑定数，禁用与缺省兜底）、
 * 基本信息五行、Schema 预览（title 兜底 key、type Tag、required 标、description、
 * 无 schema 空态）、绑定表（行渲染 + 空态文案）、工具栏四动作（健康检查文案、
 * 测试连接、运行能力 Modal 两态、保存配置 JSON 双解析校验 + 载荷 + onSaved 链）、
 * canExtensionsManage=false 只读形态、关闭回调与 open/row 守卫。
 *
 * mock 口径：services/api/extensions 七个函数 jest.mock；adapters 走真实实现
 * （纯函数，检验 adapter 兜底翼的真实行为）；SummaryOverview 真实渲染
 * （纯展示，items 文本直出）。@umijs/max 本地 mock（FormattedMessage/useIntl
 * 带 values 插值、useAccess）。
 *
 * 边界（诚实）：
 * 1. 健康检查/测试连接/保存配置的 onClick `if (!target) return` true 翼经 UI
 *    不可达——按钮仅在 `!loading && target` 内容区外的 extra 里，但 target 未
 *    就绪时点击面板仍可触发；target 恒由加载链回填，不可达场景为 detail 接口
 *    reject 后 target 仍 undefined（用例不造假，登记）。
 * 2. 加载链 finally 的 cancelled=true 翼（卸载竞态）不造假断言。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import InstallationDetailDrawer from '../InstallationDetailDrawer';
import type { ExtensionBindingItem, ExtensionInstallationItem } from '@/services/api/extensions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/extensions', () => ({
  getExtensionInstallationDetail: jest.fn(),
  getExtensionConfigSchema: jest.fn(),
  getExtensionConfig: jest.fn(),
  runExtensionHealthCheck: jest.fn(),
  testExtensionConnection: jest.fn(),
  updateExtensionConfig: jest.fn(),
  getExtensionCapabilities: jest.fn(),
}));

// mock* 前缀变量：babel-jest hoist 白名单，允许 mock 工厂延迟绑定
const mockCanManage = jest.fn(() => true);

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
    values,
  }: {
    defaultMessage?: string;
    values?: Record<string, unknown>;
  }) => {
    let text = defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
    }
    return <>{text}</>;
  },
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return text;
    },
  }),
  useAccess: () => ({ canExtensionsManage: mockCanManage() }),
}));

import {
  getExtensionCapabilities,
  getExtensionConfig,
  getExtensionConfigSchema,
  getExtensionInstallationDetail,
  runExtensionHealthCheck,
  testExtensionConnection,
  updateExtensionConfig,
} from '@/services/api/extensions';

const mDetail = getExtensionInstallationDetail as jest.MockedFunction<
  typeof getExtensionInstallationDetail
>;
const mSchema = getExtensionConfigSchema as jest.MockedFunction<typeof getExtensionConfigSchema>;
const mConfig = getExtensionConfig as jest.MockedFunction<typeof getExtensionConfig>;
const mHealth = runExtensionHealthCheck as jest.MockedFunction<typeof runExtensionHealthCheck>;
const mTest = testExtensionConnection as jest.MockedFunction<typeof testExtensionConnection>;
const mUpdate = updateExtensionConfig as jest.MockedFunction<typeof updateExtensionConfig>;
const mCaps = getExtensionCapabilities as jest.MockedFunction<typeof getExtensionCapabilities>;

const item: ExtensionInstallationItem = {
  id: 7,
  installationKey: 'chatops-demo',
  extensionId: 'chatops',
  displayName: 'ChatOps',
  releaseVersion: '1.4.0',
  scopeType: 'game',
  scopeId: 'demo',
  targetType: 'agent',
  targetId: 'agent-1',
  status: 'installed',
  desiredState: 'active',
  enabled: true,
  healthStatus: 'healthy',
  lastError: '',
  updatedAt: 1727500000,
};

const bindings: ExtensionBindingItem[] = [
  {
    bindingType: 'function',
    bindingKey: 'chat.echo',
    targetRef: 'fn://chat.echo',
    status: 'active',
    lastError: '',
  },
  {
    bindingType: 'event',
    bindingKey: 'chat.send',
    targetRef: 'ev://chat.send',
    status: 'error',
    lastError: 'boom',
  },
];

const schema = {
  properties: {
    endpoint: { type: 'string', title: '端点', description: '服务地址' },
    retries: { type: 'number' },
  },
  required: ['endpoint'],
};

function renderDrawer(over?: { open?: boolean; row?: ExtensionInstallationItem | null }) {
  const onClose = jest.fn();
  const onSaved = jest.fn().mockResolvedValue(undefined);
  const utils = render(
    <App>
      <InstallationDetailDrawer
        open={over?.open ?? true}
        row={over?.row !== undefined ? over.row : item}
        onClose={onClose}
        onSaved={onSaved}
      />
    </App>,
  );
  return { ...utils, onClose, onSaved };
}

/** 打开抽屉并等待加载完成（标题出现即 target 就绪） */
async function openAndWait() {
  expect(await screen.findByText('安装详情: ChatOps')).toBeInTheDocument();
}

beforeEach(() => {
  jest.clearAllMocks();
  mockCanManage.mockReturnValue(true);
  mDetail.mockResolvedValue({ installation: item, bindings });
  mSchema.mockResolvedValue({ schema });
  mConfig.mockResolvedValue({ config: { endpoint: 'http://x' }, secretRefs: { apiKey: 'ref/k' } });
  mHealth.mockResolvedValue({ status: 'ok', checkedAt: 1 });
  mTest.mockResolvedValue({ status: 'ok' });
  mUpdate.mockResolvedValue({ status: 'ok' });
  mCaps.mockResolvedValue({ capabilities: ['cap.echo', 'cap.broadcast'] });
});

describe('InstallationDetailDrawer 加载与渲染', () => {
  it('打开加载链：标题、概览四项、基本信息五行、Schema 预览（title/type/required/description + key 兜底）、配置回显、绑定表', async () => {
    renderDrawer();
    await openAndWait();

    // 概览四项（SummaryOverview items 文本直出）
    expect(screen.getByText('已启用')).toBeInTheDocument();
    expect(screen.getByText('健康 healthy')).toBeInTheDocument();
    expect(screen.getByText('版本 1.4.0')).toBeInTheDocument();
    expect(screen.getByText('绑定 2')).toBeInTheDocument();

    // 基本信息五行
    expect(screen.getByText('#7')).toBeInTheDocument();
    expect(screen.getByText('ChatOps (chatops)')).toBeInTheDocument();
    expect(screen.getByText('game:demo')).toBeInTheDocument();
    expect(screen.getByText('agent:agent-1')).toBeInTheDocument();

    // Schema 预览：title 直出 + type Tag + required 标 + description；title 缺省兜底 key
    expect(screen.getByText('端点')).toBeInTheDocument();
    expect(screen.getByText('服务地址')).toBeInTheDocument();
    expect(screen.getByText('required')).toBeInTheDocument();
    expect(screen.getAllByText('string')).toHaveLength(1);
    expect(screen.getByText('number')).toBeInTheDocument();
    expect(screen.getByText('retries')).toBeInTheDocument();

    // 配置/密文 JSON 回显（getExtensionConfig 优先，adapter 兜底不触达；
    // getByDisplayValue 对 node.value 折叠空白但期望串不折叠，multiline JSON 用正则）
    expect(screen.getByDisplayValue(/"endpoint": "http:\/\/x"/)).toBeInTheDocument();
    expect(screen.getByDisplayValue(/"apiKey": "ref\/k"/)).toBeInTheDocument();

    // 绑定表两行：Type/Key/Target/Status/Error
    expect(screen.getByText('chat.echo')).toBeInTheDocument();
    expect(screen.getByText('fn://chat.echo')).toBeInTheDocument();
    expect(screen.getByText('boom')).toBeInTheDocument();
  });

  it('禁用/异常兜底形态：enabled=false、healthStatus/targetId/releaseVersion 缺省兜底', async () => {
    mDetail.mockResolvedValue({
      installation: {
        ...item,
        enabled: false,
        healthStatus: '',
        targetId: '',
        releaseVersion: '',
      },
      bindings: [],
    });
    renderDrawer();

    expect(await screen.findByText('已禁用')).toBeInTheDocument();
    expect(screen.getByText('健康 -')).toBeInTheDocument();
    expect(screen.getByText('版本 -')).toBeInTheDocument();
    expect(screen.getByText('agent:-')).toBeInTheDocument();
    expect(screen.getByText('绑定 0')).toBeInTheDocument();
    // 绑定表空态文案
    expect(
      screen.getByText('当前没有运行绑定数据。如果安装未生效，先执行健康检查或重建绑定。'),
    ).toBeInTheDocument();
  });

  it('schema/config 各自失败静默兜底：空态文案 + adapter 兜底配置（`{}` 文本）', async () => {
    mSchema.mockRejectedValue(new Error('schema down'));
    mConfig.mockRejectedValue(new Error('config down'));
    mDetail.mockResolvedValue({
      installation: item,
      bindings,
      config: { from: 'detail' },
      secretRefs: { k: 'v' },
    });
    renderDrawer();

    expect(await screen.findByText('当前没有可参考的 schema 数据')).toBeInTheDocument();
    // config 接口失败 → adapter 兜底（detail 顶层 config/secretRefs）
    expect(screen.getByDisplayValue(/"from": "detail"/)).toBeInTheDocument();
    expect(screen.getByDisplayValue(/"k": "v"/)).toBeInTheDocument();
    // adapter 兜底空对象翼：detail 也不带 → '{}' 文本
  });

  it('adapter 兜底空对象翼：detail 无 config/secretRefs 且接口失败 → 双 `{}` 回显', async () => {
    mSchema.mockRejectedValue(new Error('schema down'));
    mConfig.mockRejectedValue(new Error('config down'));
    mDetail.mockResolvedValue({ installation: item });
    renderDrawer();

    expect(await screen.findByText('当前没有可参考的 schema 数据')).toBeInTheDocument();
    expect(screen.getAllByDisplayValue('{}')).toHaveLength(2);
  });

  it('open/row 守卫：row=null 或 open=false 不触发加载链', async () => {
    renderDrawer({ open: true, row: null });
    renderDrawer({ open: false, row: item });
    await new Promise((r) => setTimeout(r, 50));
    expect(mDetail).not.toHaveBeenCalled();
    expect(mSchema).not.toHaveBeenCalled();
    expect(mConfig).not.toHaveBeenCalled();
  });

  it('关闭回调：点击抽屉关闭按钮 → onClose', async () => {
    const inst = renderDrawer();
    await openAndWait();
    fireEvent.click(document.querySelector('.ant-drawer-close') as HTMLElement);
    await waitFor(() => expect(inst.onClose).toHaveBeenCalledTimes(1));
  });
});

describe('InstallationDetailDrawer 工具栏动作', () => {
  it('健康检查：runExtensionHealthCheck + 「健康检查完成: ok」；无 status 响应兜底 unknown', async () => {
    renderDrawer();
    await openAndWait();
    fireEvent.click(screen.getByRole('button', { name: '健康检查' }));

    await waitFor(() => expect(mHealth).toHaveBeenCalledWith(7));
    expect(await screen.findByText('健康检查完成: ok')).toBeInTheDocument();

    // 响应无 status：`?? 'unknown'` 右翼
    mHealth.mockResolvedValue({} as never);
    fireEvent.click(screen.getByRole('button', { name: '健康检查' }));
    expect(await screen.findByText('健康检查完成: unknown')).toBeInTheDocument();
  });

  it('测试连接：testExtensionConnection + 「连接测试通过」', async () => {
    renderDrawer();
    await openAndWait();
    fireEvent.click(screen.getByRole('button', { name: '测试连接' }));

    await waitFor(() => expect(mTest).toHaveBeenCalledWith(7));
    expect(await screen.findByText('连接测试通过')).toBeInTheDocument();
  });

  it('查看运行能力：Modal tags 两态（有 tags / 空态「暂无能力数据」）', async () => {
    renderDrawer();
    await openAndWait();
    fireEvent.click(screen.getByRole('button', { name: '查看运行能力' }));

    await waitFor(() => expect(mCaps).toHaveBeenCalledWith(7));
    const modal = (await screen.findByText('扩展能力列表')).closest('.ant-modal') as HTMLElement;
    expect(modal).not.toBeNull();
    await waitFor(() => {
      expect(modal.textContent).toContain('cap.echo');
      expect(modal.textContent).toContain('cap.broadcast');
    });

    // 关闭后再次打开：响应无 capabilities 键（`|| []` 右翼）→ 空态文案
    fireEvent.click(modal.querySelector('.ant-modal-close') as HTMLElement);
    mCaps.mockResolvedValue({} as never);
    fireEvent.click(screen.getByRole('button', { name: '查看运行能力' }));
    expect(await screen.findByText('暂无能力数据')).toBeInTheDocument();
  });

  it('保存配置：修改 JSON 后 updateExtensionConfig 载荷 + 「配置已保存」+ onSaved', async () => {
    renderDrawer();
    await openAndWait();
    const configArea = screen.getByDisplayValue(/"endpoint": "http:\/\/x"/);
    fireEvent.change(configArea, { target: { value: '{"endpoint":"http://y","tls":true}' } });
    fireEvent.click(screen.getByRole('button', { name: '保存配置' }));

    await waitFor(() =>
      expect(mUpdate).toHaveBeenCalledWith(7, {
        config: { endpoint: 'http://y', tls: true },
        secretRefs: { apiKey: 'ref/k' },
      }),
    );
    expect(await screen.findByText('配置已保存')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: '保存配置' })).toBeEnabled());
  });

  it('保存校验两翼：config JSON 非法 → 「配置 JSON 格式错误」；合法后 secretRefs 非法 → 「SecretRefs JSON 格式错误」；均不调用保存', async () => {
    renderDrawer();
    await openAndWait();
    const configArea = screen.getByDisplayValue(/"endpoint": "http:\/\/x"/);
    const secretArea = screen.getByDisplayValue(/"apiKey": "ref\/k"/);

    // config 非法翼
    fireEvent.change(configArea, { target: { value: '{oops' } });
    fireEvent.click(screen.getByRole('button', { name: '保存配置' }));
    expect(await screen.findByText('配置 JSON 格式错误')).toBeInTheDocument();
    expect(mUpdate).not.toHaveBeenCalled();

    // config 合法 + secretRefs 非法翼
    fireEvent.change(configArea, { target: { value: '{"endpoint":"http://z"}' } });
    fireEvent.change(secretArea, { target: { value: 'not-json' } });
    fireEvent.click(screen.getByRole('button', { name: '保存配置' }));
    expect(await screen.findByText('SecretRefs JSON 格式错误')).toBeInTheDocument();
    expect(mUpdate).not.toHaveBeenCalled();
  });
});

describe('InstallationDetailDrawer 分支补充', () => {
  it('加载未就绪点击守卫：detail 未决时按钮 onClick `if (!target) return` 不触达服务', async () => {
    let resolveDetail: (v: unknown) => void = () => {};
    mDetail.mockImplementation(
      () =>
        new Promise((res) => {
          resolveDetail = res;
        }),
    );
    renderDrawer();
    // extra 按钮在内容区就绪前即可见（Drawer 常驻渲染）
    await screen.findByRole('button', { name: '健康检查' });

    fireEvent.click(screen.getByRole('button', { name: '健康检查' }));
    fireEvent.click(screen.getByRole('button', { name: '查看运行能力' }));
    fireEvent.click(screen.getByRole('button', { name: '测试连接' }));
    fireEvent.click(screen.getByRole('button', { name: '保存配置' }));
    expect(mHealth).not.toHaveBeenCalled();
    expect(mCaps).not.toHaveBeenCalled();
    expect(mTest).not.toHaveBeenCalled();
    expect(mUpdate).not.toHaveBeenCalled();
    expect(screen.queryByText('扩展能力列表')).not.toBeInTheDocument();

    // 放行加载链：标题就绪，动作恢复可用
    resolveDetail({ installation: item, bindings });
    expect(await screen.findByText('安装详情: ChatOps')).toBeInTheDocument();
  });

  it('displayName 空兜底：标题与基本信息回退 extensionId（`|| extensionId` 右翼）', async () => {
    mDetail.mockResolvedValue({ installation: { ...item, displayName: '' }, bindings });
    renderDrawer();

    expect(await screen.findByText('安装详情: chatops')).toBeInTheDocument();
    expect(screen.getByText('chatops (chatops)')).toBeInTheDocument();
  });

  it('schema 变体：字段值 null（`(raw || {})` 右翼）、type 缺省（any）、无 required 键', async () => {
    mSchema.mockResolvedValue({ schema: { properties: { broken: null } } });
    renderDrawer();

    expect(await screen.findByText('broken')).toBeInTheDocument();
    expect(screen.getByText('any')).toBeInTheDocument();
    expect(screen.queryByText('required')).not.toBeInTheDocument();
    expect(screen.queryByText('端点')).not.toBeInTheDocument();
  });

  it("空 config/secretRefs 保存：`JSON.parse(... || '{}')` 双右翼 → 载荷均为空对象", async () => {
    renderDrawer();
    await openAndWait();
    fireEvent.change(screen.getByDisplayValue(/"endpoint": "http:\/\/x"/), {
      target: { value: '' },
    });
    fireEvent.change(screen.getByDisplayValue(/"apiKey": "ref\/k"/), {
      target: { value: '' },
    });
    fireEvent.click(screen.getByRole('button', { name: '保存配置' }));

    await waitFor(() => expect(mUpdate).toHaveBeenCalledWith(7, { config: {}, secretRefs: {} }));
    expect(await screen.findByText('配置已保存')).toBeInTheDocument();
  });

  it('卸载竞态：detail 未决时卸载 → resolve 后 cancelled 翼直接返回，无状态更新崩溃', async () => {
    let resolveDetail: (v: unknown) => void = () => {};
    mDetail.mockImplementation(
      () =>
        new Promise((res) => {
          resolveDetail = res;
        }),
    );
    const inst = renderDrawer();
    await new Promise((r) => setTimeout(r, 30));
    inst.unmount();
    resolveDetail({ installation: item, bindings });
    await new Promise((r) => setTimeout(r, 30));
    expect(mSchema).toHaveBeenCalled(); // 加载链继续执行至 cancelled 检查后返回
  });
});

describe('InstallationDetailDrawer 权限形态', () => {
  it('canExtensionsManage=false：健康检查/测试连接/保存配置禁用，查看运行能力可用', async () => {
    mockCanManage.mockReturnValue(false);
    renderDrawer();
    await openAndWait();

    expect(screen.getByRole('button', { name: '健康检查' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '测试连接' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '保存配置' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '查看运行能力' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: '健康检查' }));
    fireEvent.click(screen.getByRole('button', { name: '保存配置' }));
    expect(mHealth).not.toHaveBeenCalled();
    expect(mUpdate).not.toHaveBeenCalled();
  });
});
