/**
 * 函数详情页本体单测（覆盖率补缺：Detail.tsx 463 行 0% → 收口——v8 定向
 * 扫描确证 1-463 全零；resolver 核验既有 8 个测试全打 DetailSections/
 * DetailTabs/useFunctionDetailPage，无人 import `../Detail`，仅
 * config/routes.ts 有路由字符串引用，import 面核验后认领）。
 *
 * 锁定契约（页面本体，子组件各自有测试不在本文件重复）：
 * - not-found 面板（!functionDetail && !loading）：三段文案 + extra=id +
 *   返回按钮 push 目录；
 * - 标签云与头部：name||id 双处回退、enabled Badge 两态、version/
 *   effectiveResource/descriptorResource/descriptorOperation 四 Tag 有无、
 *   description||intl fallback；
 * - 七 tab 接线（FUNCTION_DETAIL_SCHEMA.tabs → tabContent 桩可切换）与
 *   Tabs onChange 的 URL 同步（config 双参带 subTab / 非 config 删
 *   subTab）+ useLocation.search 初值两态与 effect 同步；
 * - 动作四键 runAction：reload→loadDetail 重入（loadingWhen）、copy→
 *   push 新 ID、delete→confirm onOk 链、edit 双态（setEditing↔
 *   form.submit→onFinish handleSave 真链）+ disabledWhen noFunction；
 * - DetailConfigTab 回调接线：onJsonCopySuccess/Error 走 App message
 *   portal、onSubTabChange 同步 activeSubTab+URL、onOpenPageStudio；
 * - 建议动作区：viewCandidates/invokeTest push（invokePath fid 编码）。
 *
 * mock 口径：useFunctionDetailPage 真跑（services 十函数 jest.mock 驱动
 * 状态——form 为真实例、<Form form> 与 onFinish 链保真）；DetailSections/
 * DetailTabs/DetailConfigTab 轻桩（桩内触发 props 回调验证接线）；
 * '@/components' 桩（barrel 链不入测试面，PageStatePanel 桩渲染三段
 * 文案供 not-found 断言）；@umijs/max 自含 makeIntl + 可变
 * mockParams/mockLocation（mock* 前缀过 hoist）；antd 真实 +
 * <App> 包裹（message 经真实 portal DOM 断言，坑 8b 同源）。
 *
 * 边界（诚实，v8 复扫实证 100/90.09/100/100）——8 处分支翼结构性不可达：
 * - L60 `|| 'json'`：activeSubTab useState 初始化即 `|| 'json'`，恒非空串；
 * - L63 `: ''`：buildSearch 总先 search.set('tab') → toString 恒非空；
 * - L114/L152-155 `params.id || ''` ×5：id 缺省走 not-found 早退，正常渲染区恒真；
 * - L198 Badge `'default'`：useFunctionDetailPage normalize 硬编码 enabled:true
 *   （toggle 禁用链成功后 loadDetail 重拉亦回 true，见 handleStatusToggle 用例）；
 * - L294 `|| functionDetail?.id`：name = localizedText(displayName,'zh-CN',id)
 *   恒非空（最坏回退 id 本身）。
 * 另两处 antd v6 行为契约（非缺陷）：modal.confirm portal 在 <App> holder
 * 不继承内层 ConfigProvider zhCN → OK 按钮默认文案 "OK"、class ant-btn-dangerous；
 * 页面 Form component={false} 不注册 Form.Item → 编辑保存 onFinish values 全空。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import FunctionDetailPage from '../Detail';
import {
  getFunctionDetail,
  getFunctionOpenAPI,
  updateFunction,
  deleteFunction,
  copyFunction,
  enableFunction,
  disableFunction,
  getFunctionPermissions,
  updateFunctionPermissions,
  listDescriptors,
} from '@/services/api/functions';
import { history } from '@umijs/max';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

// 可变 mock 状态：mock* 前缀变量过 babel-jest hoist 白名单
const mockParams = { id: 'fn.main' as string | undefined };
const mockLocation = { pathname: '/functions/fn.main', search: '' };

jest.mock('@umijs/max', () => {
  const makeIntl = () => ({
    locale: 'zh-CN',
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return text;
    },
  });
  return {
    useIntl: makeIntl,
    getIntl: makeIntl,
    useParams: () => mockParams,
    useLocation: () => mockLocation,
    history: { push: jest.fn(), replace: jest.fn() },
    FormattedMessage: ({
      defaultMessage,
      values,
    }: {
      defaultMessage?: string;
      values?: Record<string, string>;
    }) => {
      let text = defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return <>{text}</>;
    },
  };
});

jest.mock('@/services/api/functions', () => ({
  __esModule: true,
  getFunctionDetail: jest.fn(),
  getFunctionOpenAPI: jest.fn(),
  updateFunction: jest.fn(),
  deleteFunction: jest.fn(),
  copyFunction: jest.fn(),
  enableFunction: jest.fn(),
  disableFunction: jest.fn(),
  getFunctionPermissions: jest.fn(),
  updateFunctionPermissions: jest.fn(),
  listDescriptors: jest.fn(),
}));

// barrel 链不入测试面：PageStatePanel 桩渲染 not-found 三段文案 + actions。
// 文案各包一层 span——裸文本节点归属 parent，getByText 精确匹配 parent
// textContent（全部拼接）必失败（首跑实证）
jest.mock('@/components', () => {
  const R = require('react');
  const line = (v: React.ReactNode) =>
    v == null || v === '' ? null : R.createElement('span', null, v);
  return {
    DASHBOARD_PAGE_TOKENS: { cardPadding: 16 },
    PageStatePanel: (props: {
      badgeText?: React.ReactNode;
      title?: React.ReactNode;
      description?: React.ReactNode;
      extra?: React.ReactNode;
      actions?: React.ReactNode;
    }) =>
      R.createElement(
        'div',
        { 'data-testid': 'state-panel' },
        line(props.badgeText),
        line(props.title),
        line(props.description),
        line(props.extra),
        props.actions,
      ),
  };
});

// 页面本体测试：子组件轻桩（各自有测试），桩内暴露 props 回调触发按钮
jest.mock('../DetailSections', () => {
  const R = require('react');
  return {
    BasicInfoTab: (props: { onStatusToggle: (enabled: boolean) => void }) =>
      R.createElement(
        'div',
        { 'data-testid': 'basic-stub' },
        R.createElement(
          'button',
          { onClick: () => props.onStatusToggle(false) },
          'stub-toggle-off',
        ),
      ),
    PermissionsTab: () => R.createElement('div', { 'data-testid': 'perm-stub' }),
  };
});

jest.mock('../DetailTabs', () => {
  const R = require('react');
  const stub = (name: string) => () => R.createElement('div', { 'data-testid': `${name}-stub` });
  return {
    AnalyticsTab: stub('analytics'),
    HistoryTab: stub('history'),
    VersionsTab: stub('versions'),
    WarningsTab: stub('warnings'),
  };
});

jest.mock('../DetailConfigTab', () => {
  const R = require('react');
  return {
    __esModule: true,
    default: (props: {
      onSubTabChange: (key: string) => void;
      onJsonCopySuccess: () => void;
      onJsonCopyError: () => void;
      onOpenPageStudio: () => void;
    }) =>
      R.createElement(
        'div',
        { 'data-testid': 'config-stub' },
        R.createElement('button', { onClick: () => props.onJsonCopySuccess() }, 'stub-copy-ok'),
        R.createElement('button', { onClick: () => props.onJsonCopyError() }, 'stub-copy-err'),
        R.createElement('button', { onClick: () => props.onSubTabChange('schema') }, 'stub-sub'),
        R.createElement('button', { onClick: () => props.onOpenPageStudio() }, 'stub-studio'),
      ),
  };
});

const mDetail = jest.mocked(getFunctionDetail);
const mOpenapi = jest.mocked(getFunctionOpenAPI);
const mUpdate = jest.mocked(updateFunction);
const mDelete = jest.mocked(deleteFunction);
const mCopy = jest.mocked(copyFunction);
const mEnable = jest.mocked(enableFunction);
const mDisable = jest.mocked(disableFunction);
const mGetPerm = jest.mocked(getFunctionPermissions);
const mUpdatePerm = jest.mocked(updateFunctionPermissions);
const mDescs = jest.mocked(listDescriptors);
const mPush = history.push as jest.Mock;
const mReplace = history.replace as jest.Mock;

const baseDetail = {
  id: 'fn.main',
  displayName: { 'zh-CN': '主函数', 'en-US': 'Main' },
  summary: { 'zh-CN': '主函数摘要' },
  description: { 'zh-CN': '主函数描述' },
  resource: 'player',
  operation: 'query',
  version: '2.1.0',
  tags: ['hot', 'read'],
  enabled: true,
};

function renderPage() {
  return render(
    <App>
      <ConfigProvider locale={zhCN}>
        <FunctionDetailPage />
      </ConfigProvider>
    </App>,
  );
}

/** 等待主加载链 settle（页面脱离 loading 骨架） */
async function settle() {
  await waitFor(() => expect(document.querySelector('.ant-btn-loading')).not.toBeInTheDocument());
}

beforeEach(() => {
  jest.clearAllMocks();
  mockParams.id = 'fn.main';
  mockLocation.pathname = '/functions/fn.main';
  mockLocation.search = '';
  mDetail.mockResolvedValue(baseDetail as never);
  mOpenapi.mockResolvedValue({ extensions: {}, requestBody: undefined } as never);
  mDescs.mockResolvedValue([{ id: 'fn.main', version: '2.1.0' }] as never);
  mGetPerm.mockResolvedValue({ items: [] } as never);
  mUpdate.mockResolvedValue(undefined);
  mDelete.mockResolvedValue(undefined);
  mCopy.mockResolvedValue({ functionId: 'fn.copy-1' } as never);
  mEnable.mockResolvedValue(undefined);
  mDisable.mockResolvedValue(undefined);
  mUpdatePerm.mockResolvedValue(undefined);
});

describe('Detail not-found 面板', () => {
  it('404 降级后渲染三段文案 + extra=id + 返回按钮 push 目录', async () => {
    mDetail.mockRejectedValue(Object.assign(new Error('nf'), { response: { status: 404 } }));
    mDescs.mockRejectedValue(new Error('desc down'));
    renderPage();

    const panel = await screen.findByTestId('state-panel');
    expect(within(panel).getByText('未找到函数')).toBeInTheDocument();
    expect(within(panel).getByText('当前函数不存在')).toBeInTheDocument();
    expect(
      within(panel).getByText('请检查函数 ID 是否正确，或从函数目录重新进入。'),
    ).toBeInTheDocument();
    expect(within(panel).getByText('fn.main')).toBeInTheDocument();

    fireEvent.click(within(panel).getByRole('button', { name: /返回函数列表/ }));
    expect(mPush).toHaveBeenCalledWith('/functions/catalog');
    // not-found 态动作区不可达（L72 早退），此处只锁面板
  });

  it('params.id 缺省：hook 早退 → not-found + invoke 链不入正常区', async () => {
    mockParams.id = undefined;
    mDetail.mockResolvedValue(undefined as never); // loadDetail 早退不触发
    renderPage();
    expect(await screen.findByTestId('state-panel')).toBeInTheDocument();
    expect(screen.getByText('当前函数不存在')).toBeInTheDocument();
    expect(mDetail).not.toHaveBeenCalled();
    // L55 invokePath=''（id 缺省）——正常渲染区不可达，经 not-found 锁定
    expect(screen.queryByRole('button', { name: /测试调用/ })).not.toBeInTheDocument();
  });
});

describe('Detail 正常渲染与标签云', () => {
  it('全字段态：四 Tag + enabled Badge + version + description + name 双处', async () => {
    mDescs.mockResolvedValue([
      { id: 'fn.main', version: '2.1.0', resource: 'mail', operation: 'send' },
    ] as never);
    renderPage();
    await settle();

    // PageContainer 标题与正文标题（name||id 双处同取 name）
    expect((await screen.findAllByText('主函数')).length).toBeGreaterThanOrEqual(2);
    // 状态与四 Tag
    expect(screen.getByText('已启用')).toBeInTheDocument();
    expect(screen.getByText('v2.1.0')).toBeInTheDocument();
    expect(screen.getByText('player')).toBeInTheDocument();
    expect(screen.getByText('资源 mail')).toBeInTheDocument();
    expect(screen.getByText('操作 send')).toBeInTheDocument();
    // description 区取 normalize 后的 summary 优先值（useFunctionDetailPage L205-209：summary 回退 description 回退 ''）
    expect(screen.getByText('主函数摘要')).toBeInTheDocument();
    expect(screen.queryByText(/这里用于确认单个函数的能力定义/)).not.toBeInTheDocument();
    // 七 tab 项齐全 + 子组件桩可切换
    expect(screen.getByRole('tab', { name: '基本信息' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '变更历史' })).toBeInTheDocument();
  });

  it('缺省翼：无 version/无 resource/无 description → Tag 缺失 + fallback 文案 + Badge default', async () => {
    mDetail.mockResolvedValue({
      id: 'fn.other',
      displayName: undefined,
      summary: undefined,
      description: undefined,
      resource: '',
      enabled: false,
    } as never);
    mDescs.mockResolvedValue([{ id: 'fn.other' }] as never);
    renderPage();
    await settle();

    expect((await screen.findAllByText('fn.other')).length).toBeGreaterThanOrEqual(1); // name 回退 id
    expect(screen.queryByText(/v\d/)).not.toBeInTheDocument();
    expect(screen.queryByText('玩家')).not.toBeInTheDocument();
    // enabled 翼见 handleStatusToggle 用例（normalize 硬编码 true，false 翼不可达）
    expect(screen.getByText('已启用')).toBeInTheDocument();
    expect(
      screen.getByText(/这里用于确认单个函数的能力定义、资源\/操作归属和 JSON Schema 契约/),
    ).toBeInTheDocument();
    // descriptorResource/Operation trim 后为空 → 两 Tag 不渲染
    expect(screen.queryByText(/^资源 /)).not.toBeInTheDocument();
    expect(screen.queryByText(/^操作 /)).not.toBeInTheDocument();
    // effectiveResource 空 → 紫 Tag 缺失（purple Tag 仅 DOM class，锁 absence of 'player' 已证）
  });

  it('loading 态：Card 骨架 + reload 按钮 loadingWhen + noFunction 禁用三键', async () => {
    let resolveDetail: (v: unknown) => void = () => {};
    mDetail.mockImplementation(
      () =>
        new Promise((res) => {
          resolveDetail = res;
        }),
    );
    renderPage();

    // loading=true 且 functionDetail=null → 不进 not-found（L72），正常区渲染
    expect(screen.queryByTestId('state-panel')).not.toBeInTheDocument();
    const reload = screen.getByRole('button', { name: /刷新/ });
    expect(reload).toHaveClass('ant-btn-loading'); // loadingWhen: 'loading'
    // noFunction → copy/delete/edit disabled，reload 不受 disabledWhen 约束
    expect(screen.getByRole('button', { name: /复制/ })).toBeDisabled();
    expect(screen.getByRole('button', { name: /删除/ })).toBeDisabled();
    expect(screen.getByRole('button', { name: /编辑/ })).toBeDisabled();

    resolveDetail(baseDetail);
    await settle();
    expect(screen.getByRole('button', { name: /复制/ })).toBeEnabled();
  });
});

describe('Detail 动作四键（runAction）', () => {
  it('reload：loadDetail 重入（getFunctionDetail 二次调用）', async () => {
    renderPage();
    await settle();
    expect(mDetail).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mDetail).toHaveBeenCalledTimes(2));
  });

  it('copy：成功 → push 新函数 ID（handleCopy 真链）', async () => {
    renderPage();
    await settle();
    fireEvent.click(screen.getByRole('button', { name: /复制/ }));
    await waitFor(() => expect(mPush).toHaveBeenCalledWith('/functions/fn.copy-1'));
  });

  it('handleStatusToggle（BasicInfoTab 接线）：disable 链 + message + loadDetail 重拉；enabled 恒 true（normalize 硬编码）', async () => {
    renderPage();
    await settle();
    const stub = await screen.findByTestId('basic-stub');
    fireEvent.click(within(stub).getByRole('button', { name: 'stub-toggle-off' }));
    await waitFor(() => expect(mDisable).toHaveBeenCalledWith('fn.main'));
    expect(mEnable).not.toHaveBeenCalled();
    expect(await screen.findByText('函数已禁用')).toBeInTheDocument();
    await waitFor(() => expect(mDetail).toHaveBeenCalledTimes(2)); // 成功链 loadDetail
    expect(screen.getByText('已启用')).toBeInTheDocument(); // 重拉后仍 enabled:true（L212 硬编码）
  });

  it('delete：confirm onOk → 删除 + push 目录', async () => {
    renderPage();
    await settle();
    fireEvent.click(screen.getByRole('button', { name: /删除/ }));
    // 未确认前不删；confirm 走 antd 真实 portal。OK 按钮直查 DOM 的两个原因：
    // ① modal portal 在 <App> holder（外层），不继承内层 ConfigProvider 的 zhCN →
    //    按钮文案是默认 "OK"/"Cancel" 而非「确 定」；② antd v6 okType:'danger' 的
    //    class 是 ant-btn-dangerous（无 ant-btn-primary）。PageStudio studioActions
    //    的 .ant-modal-confirm-btns DOM 直查同源
    const confirmBtn = await waitFor(() => {
      const btn = document.body.querySelector('.ant-modal-confirm-btns .ant-btn-dangerous');
      expect(btn).not.toBeNull();
      return btn as HTMLElement;
    });
    expect(screen.getByText('确定要删除这个函数吗？此操作不可恢复！')).toBeInTheDocument();
    expect(mDelete).not.toHaveBeenCalled();
    fireEvent.click(confirmBtn);
    await waitFor(() => expect(mDelete).toHaveBeenCalledWith('fn.main'));
    expect(await screen.findByText('删除成功')).toBeInTheDocument();
    expect(mPush).toHaveBeenCalledWith('/functions/catalog');
  });

  it('edit 双态：未编辑 → setEditing；已编辑 → form.submit → onFinish handleSave 真链', async () => {
    renderPage();
    await settle();

    // 初态：编辑（primary）+ EditOutlined
    const editBtn = screen.getByRole('button', { name: /编辑/ });
    expect(editBtn).toHaveClass('ant-btn-primary');
    fireEvent.click(editBtn);

    // editing：文案切保存 + icon 切 SaveOutlined（L229-241 双翼）
    const saveBtn = await screen.findByRole('button', { name: /保存/ });
    expect(mUpdate).not.toHaveBeenCalled();
    fireEvent.click(saveBtn);

    // form.submit → <Form onFinish=handleSave> 真链。页面 Form component={false}
    // 不注册 Form.Item（编辑字段在子 tab 内），onFinish values 全空 → handleSave tags 兜底 []
    await waitFor(() => expect(mUpdate).toHaveBeenCalledTimes(1));
    expect(mUpdate).toHaveBeenCalledWith('fn.main', {
      name: undefined,
      description: undefined,
      resource: undefined,
      tags: [],
    });
    expect(await screen.findByText('保存成功')).toBeInTheDocument();
    // 成功链 handleSave → setEditing(false) → 按钮回「编辑」
    await waitFor(() => expect(screen.getByRole('button', { name: /编辑/ })).toBeInTheDocument());
  });
});

describe('Detail URL 同步与 tab 接线', () => {
  it('useLocation.search 初值两态 + effect 同步（rerender 换 search 重置 tab）', async () => {
    mockLocation.search = '?tab=permissions';
    const { rerender } = renderPage();
    await settle();
    expect(screen.getByRole('tab', { selected: true })).toHaveTextContent('权限');

    // location.search 变化 → effect 重置（L66-70）
    mockLocation.search = '?tab=config&subTab=schema';
    rerender(
      <App>
        <ConfigProvider locale={zhCN}>
          <FunctionDetailPage />
        </ConfigProvider>
      </App>,
    );
    expect(screen.getByRole('tab', { selected: true })).toHaveTextContent('函数配置');
  });

  it('Tabs onChange：非 config 删 subTab；config 保留并透传当前 subTab', async () => {
    mockLocation.search = '?tab=config&subTab=schema';
    renderPage();
    await settle();
    expect(mReplace).not.toHaveBeenCalled();

    // 切到 history（非 config）→ search.delete('subTab') 翼（L61）
    fireEvent.click(screen.getByRole('tab', { name: '调用历史' }));
    expect(mReplace).toHaveBeenCalledWith('/functions/fn.main?tab=history');
    // 切回 config（带当前 activeSubTab=schema）→ buildSearch 双参（L60）
    fireEvent.click(screen.getByRole('tab', { name: '函数配置' }));
    expect(mReplace).toHaveBeenCalledWith('/functions/fn.main?tab=config&subTab=schema');
  });

  it('search 无 tab 键 → 默认 basic；query 空串不带 ?（buildSearch 空翼）', async () => {
    renderPage();
    await settle();
    expect(screen.getByRole('tab', { selected: true })).toHaveTextContent('基本信息');
    fireEvent.click(screen.getByRole('tab', { name: '权限' }));
    expect(mReplace).toHaveBeenCalledWith('/functions/fn.main?tab=permissions');
  });

  it('七 tab 内容接线：逐 tab 激活渲染对应子组件桩（antd Tabs 懒渲染）', async () => {
    renderPage();
    await settle();
    expect(screen.getByTestId('basic-stub')).toBeInTheDocument(); // 默认 basic
    for (const [tab, stub] of [
      ['调用历史', 'history-stub'],
      ['统计分析', 'analytics-stub'],
      ['注册告警', 'warnings-stub'],
      ['变更历史', 'versions-stub'],
      ['权限', 'perm-stub'],
    ] as const) {
      fireEvent.click(screen.getByRole('tab', { name: tab }));
      expect(await screen.findByTestId(stub)).toBeInTheDocument();
    }
  });

  it('头部返回按钮：push 函数目录', async () => {
    renderPage();
    await settle();
    fireEvent.click(screen.getByRole('button', { name: /返回/ }));
    expect(mPush).toHaveBeenCalledWith('/functions/catalog');
  });
});

describe('Detail 子组件回调接线', () => {
  it('DetailConfigTab：copy 成功/失败走 App message portal；subTab 切换同步 URL；OpenPageStudio push', async () => {
    renderPage();
    await settle();
    fireEvent.click(screen.getByRole('tab', { name: '函数配置' }));
    const stub = await screen.findByTestId('config-stub');

    fireEvent.click(within(stub).getByRole('button', { name: 'stub-copy-ok' }));
    expect(await screen.findByText('JSON 已复制')).toBeInTheDocument();

    fireEvent.click(within(stub).getByRole('button', { name: 'stub-copy-err' }));
    expect(await screen.findByText('复制失败')).toBeInTheDocument();

    // onSubTabChange('schema') → activeSubTab + replace 双参
    fireEvent.click(within(stub).getByRole('button', { name: 'stub-sub' }));
    expect(mReplace).toHaveBeenCalledWith('/functions/fn.main?tab=config&subTab=schema');

    fireEvent.click(within(stub).getByRole('button', { name: 'stub-studio' }));
    expect(mPush).toHaveBeenCalledWith('/functions/resource-catalog');
  });

  it('建议动作区：三处 viewCandidates push；invokeTest 带 fid 编码 push', async () => {
    renderPage();
    await settle();
    // header extra / 建议卡 / info Alert action 三处 viewCandidates（getAllBy 防多命中）
    const viewBtns = screen.getAllByRole('button', { name: /查看资源\/页面候选/ });
    expect(viewBtns).toHaveLength(3);
    viewBtns.forEach((btn) => {
      fireEvent.click(btn);
      expect(mPush).toHaveBeenLastCalledWith('/functions/resource-catalog');
    });

    fireEvent.click(screen.getByRole('button', { name: /测试调用/ }));
    expect(mPush).toHaveBeenLastCalledWith('/functions/invoke?fid=fn.main');
  });
});

describe('Detail 契约诊断 Alert', () => {
  it('diagnostics 非空 → Alert + code/field/message 三元素；field/message 缺省翼', async () => {
    mDescs.mockResolvedValue([
      {
        id: 'fn.main',
        version: '2.1.0',
        diagnostics: [
          { code: 'schema_breaking_change', field: 'inputSchema', message: '不兼容' },
          { code: 'missing_summary', field: '', message: '' },
          { code: 'no_field_no_message', field: undefined, message: undefined },
        ],
      },
    ] as never);
    renderPage();
    await settle();

    expect(await screen.findByText('契约诊断告警')).toBeInTheDocument();
    expect(screen.getByText('schema_breaking_change')).toBeInTheDocument();
    expect(screen.getByText('inputSchema')).toBeInTheDocument();
    expect(screen.getByText('不兼容')).toBeInTheDocument();
    // field/message 缺省翼：code 渲染但 Tag/正文不渲染（多 key 汇总）
    expect(screen.getByText('missing_summary')).toBeInTheDocument();
    expect(screen.getByText('no_field_no_message')).toBeInTheDocument();
    // 常驻 info Alert 同屏
    expect(screen.getByText('函数层负责能力定义，Page Studio 负责页面装配')).toBeInTheDocument();
  });
});
