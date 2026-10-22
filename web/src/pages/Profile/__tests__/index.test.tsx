/**
 * 个人中心页单测（覆盖率补缺轮：Profile 入口 + 数据层 0% → 收口）。
 *
 * 现状：__tests__ 内六份既有套件全是单组件回归（GamesTab/InfoTab/MfaSettings/
 * NotificationsTab/PermissionTree/SecurityTab），页面入口 index.tsx（452 行）、
 * 数据层 useProfileData.ts（323 行）与 shared.ts 的页面级接线全程 0 覆盖。
 * 本套件经真实页面渲染锁定三文件契约：
 *
 * - 首拉链：loadProfile → hero 渲染矩阵（displayName/Badge/roles/Descriptions
 *   未设置兜底）+ loadExtras 七路 Promise.allSettled 并行（getMyGames/
 *   getMyPermissions({})/listAudit activities{actor,size:8}/listAudit login
 *   {actor,kinds,size:20}/listMessages{status:'all',pageSize:8}/listPermissions
 *   {page:1,pageSize:500}/fetchMyNotificationChannels）+ stats 四卡派生值。
 * - 加载/失败分支：pending 骨架（profile null 翼）、getMyProfile reject →
 *   toast + 停留骨架。
 * - Tab 编排：URL 深链初始 tab（?tab=games 时 InfoTab 不挂载）、切换时
 *   navigate replace 同步 ?tab=、hero 编辑按钮强制回资料页 + scrollIntoView。
 * - 资料编辑链（handleProfileSubmit + InfoTab 共享表单实例）：成功（载荷 +
 *   toast + 退出编辑 + 重拉）、失败（toast + 保持编辑）、校验（displayName
 *   required / phone pattern）、取消回填。
 * - 消息已读链（openMessage/markMessageRead/markAllRead 在数据层）：未读详情
 *   打开即标读 + 状态翻转、已读消息不触发、全部已读按 unread 过滤、徽标/
 *   按钮随 unreadCount 消失。
 * - 权限派生 memo：permissionGroups 同 resource+scope 并集合并、scopeless
 *   组无 Tag、applyPermissionCandidates 目录驱动（key/id 双过滤 + slice 20）、
 *   目录空列表 → 六模板 fallback 兜底但标记仍绿（catalogAvailable 只认请求
 *   成败，现状锁定）、目录请求失败 → 金色受限 Tag、申请弹窗 reason 必填 +
 *   createFeedback 载荷。
 * - loginSessionRows：pickAuditMetaValue 多键提取（ip/client_ip）、成败从
 *   kind 推断（login_fail/login_rate_limited → 失败）、latestLoginIP 首个
 *   有 IP 行进 InfoTab。
 * - loadExtras 失败矩阵：七路全 reject 走 settled 静默（不弹 extras.error），
 *   各 Tab 空态 Alert / stats 归零 / 目录回退；perms 载荷 {} 的 nullish 右翼；
 *   username 空串时 login 查询不发（Promise.resolve 右翼）。
 *
 * mock 口径：me/audit/messages/permissions/auth/support/storage 七个 service
 * 模块 jest.mock；@umijs/max 本地 mock（工厂内 require 真实 zh-CN pages
 * locale——页面 formatMessage 全部 id-only）；pro-components 仅替换
 * PageContainer（ModalForm 保持真实，PermissionsTab 申请弹窗依赖真实表单）；
 * Tabs/子组件/AuditList/SimpleList/UserAvatar/PasswordModal/AvatarModal 全部
 * 真实渲染（弹窗内部提交流属各组件自有套件域，本套件只锁开合接线）。
 *
 * 边界（诚实清单）：
 * 1. loadExtras 外层 catch（'profile.extras.error' toast）结构性不可达——
 *    Promise.allSettled 永不 reject，setStates 亦无抛出路径；七路全 reject
 *    用例实证「无 toast、页面正常落空态」。不造假用例。
 * 2. markAllRead 的 `unread.length === 0` 早退经 UI 不可达——「全部标为已读」
 *    按钮仅在 unreadCount>0 时渲染。同族：openMessage/markMessageRead 的
 *    markMessagesRead reject `.catch(() => undefined)` 静默翼（状态不翻转，
 *    现状行为，不造假 reject 场景）。
 * 3. MfaSettings/PasswordModal/AvatarModal 本体（提交/上传流）属各组件自有
 *    回归套件域（MfaSettings.test / InfoTab.regression 覆盖 normalizeAvatarSrc）；
 *    本套件经 SecurityTab/hero 入口锁开合与 onPersisted/onClose 接线。
 * 4. loginSessionRows 的 key 兜底（hash/time/idx 三级）经 rowKey 不上 DOM，
 *    以行渲染存在性锁定；idx 翼由 time 空串事件连带触达。
 * 5. 契约死翼登记（不造假用例、不删防御分支）：
 *    a. loadExtras 各 settled 取值链的 nullish 右翼（games/perms/messages/
 *       channels 由 service 归一层保证形状，listPermissions/listAudit 契约
 *       声明必填字段——构造 fulfilled-undefined/缺键形态违反返回类型）；
 *    b. Array.isArray(ids/rolePermissions/actions) 的 false 翼——
 *       getMyPermissions 归一化注释明示「缺失会被补」；
 *    c. markMessageRead 的已读早退（标读按钮仅未读行渲染）；
 *    d. detailMessage 更新器的 prev 失配翼（openMessage 先置 detail 再标读；
 *       行内标读时详情 Modal 遮罩挡住列表，两翼各自只剩一条可达路径）；
 *    e. loginRecords||[] / item.meta||{}（useState 恒数组 /
 *       normalizeAuditEvent 的 metadata ?? {} 恒对象）；
 *    f. profileText 的 number 翼（调用点类型均为 string|undefined）；
 *    g. pickAuditMetaValue 的 !meta 守卫（唯一调用方先经 (item.meta||{})
 *       归一后传入）。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import ProfilePage from '../index';
import { formatDateTime } from '@/utils/format';
import type { MeProfile, ProfileGame, ProfilePermission } from '@/services/api/me';
import type { AuditEvent } from '@/services/api/audit';
import type { MessageItem } from '@/services/api/messages';
import type { PermissionRecord } from '@/services/api/permissions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

// jsdom 未实现 scrollIntoView——hero 编辑按钮的 setTimeout 滚动需要它
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = jest.fn();
}
const scrollIntoViewMock = Element.prototype.scrollIntoView as jest.Mock;

jest.mock('@/services/api/me', () => ({
  getMyProfile: jest.fn(),
  getMyGames: jest.fn(),
  getMyPermissions: jest.fn(),
  updateMyProfile: jest.fn(),
  fetchMyNotificationChannels: jest.fn(),
  changeMyPassword: jest.fn(),
}));

jest.mock('@/services/api/audit', () => ({ listAudit: jest.fn() }));

jest.mock('@/services/api/messages', () => ({
  listMessages: jest.fn(),
  markMessagesRead: jest.fn(),
}));

jest.mock('@/services/api/permissions', () => ({ listPermissions: jest.fn() }));

jest.mock('@/services/api/auth', () => ({
  fetchMfaStatus: jest.fn(),
  setupMfa: jest.fn(),
  confirmMfa: jest.fn(),
  disableMfa: jest.fn(),
}));

jest.mock('@/services/api/support', () => ({ createFeedback: jest.fn() }));

jest.mock('@/services/api/storage', () => ({
  buildAvatarObjectKey: jest.fn(() => 'avatars/test.png'),
  uploadAsset: jest.fn(),
}));

// mock* 前缀变量：工厂闭包延迟绑定（工厂体自身不读，无 TDZ）
const mockNavigate = jest.fn();
let mockSearch = '';

jest.mock('@umijs/max', () => {
  // 工厂自包含：locale 查表在工厂体内构造（页面 formatMessage 全部 id-only，
  // 无 defaultMessage 兜底——必须用真实 zh-CN 词表）
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
    useLocation: () => ({ search: mockSearch, pathname: '/profile', hash: '' }),
    useNavigate: () => mockNavigate,
    useModel: () => ({ initialState: {}, loading: false, refresh: jest.fn() }),
  };
});

// PageContainer 换桩即可；ModalForm 保持真实（PermissionsTab 申请弹窗的
// formRef/校验/destroyOnHidden 行为依赖真实实现）
jest.mock('@ant-design/pro-components', () => ({
  ...jest.requireActual('@ant-design/pro-components'),
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

import {
  changeMyPassword,
  fetchMyNotificationChannels,
  getMyGames,
  getMyPermissions,
  getMyProfile,
  updateMyProfile,
} from '@/services/api/me';
import { listAudit } from '@/services/api/audit';
import { listMessages, markMessagesRead } from '@/services/api/messages';
import { listPermissions } from '@/services/api/permissions';
import { fetchMfaStatus } from '@/services/api/auth';
import { createFeedback } from '@/services/api/support';

const mProfile = getMyProfile as jest.MockedFunction<typeof getMyProfile>;
const mGames = getMyGames as jest.MockedFunction<typeof getMyGames>;
const mPerms = getMyPermissions as jest.MockedFunction<typeof getMyPermissions>;
const mUpdate = updateMyProfile as jest.MockedFunction<typeof updateMyProfile>;
const mChannels = fetchMyNotificationChannels as jest.MockedFunction<
  typeof fetchMyNotificationChannels
>;
const mChangePwd = changeMyPassword as jest.MockedFunction<typeof changeMyPassword>;
const mAudit = listAudit as jest.MockedFunction<typeof listAudit>;
const mMessages = listMessages as jest.MockedFunction<typeof listMessages>;
const mMarkRead = markMessagesRead as jest.MockedFunction<typeof markMessagesRead>;
const mCatalog = listPermissions as jest.MockedFunction<typeof listPermissions>;
const mMfaStatus = fetchMfaStatus as jest.MockedFunction<typeof fetchMfaStatus>;
const mFeedback = createFeedback as jest.MockedFunction<typeof createFeedback>;

// avatar 纯空白：normalizeAvatarSrc → undefined（icon/首字母占位，BUG-007）
const PROFILE: MeProfile = {
  id: 42,
  username: 'alice',
  nickname: 'Ali',
  displayName: 'Alice Admin',
  email: 'alice@example.com',
  phone: '13800138000',
  avatar: '   ',
  active: true,
  roles: ['admin', 'ops'],
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
  lastLoginAt: '2026-09-01T00:00:00Z',
};

// 三形态：scoped（有 perms 无 accessLevel）/ full（accessLevel 直给）/
// none（全空 → 「无显式权限」不留白，BUG-018）
const GAMES: ProfileGame[] = [
  { gameId: 'demo', gameName: 'Demo Game', envs: ['prod', 'demo'], permissions: ['players:read'] },
  { gameId: 'ops', gameName: '', envs: [], permissions: ['*'], accessLevel: 'full' },
  { gameId: 'empty', gameName: 'Empty One', envs: [], permissions: [] },
];

// 前两条同 resource+scope → permissionGroups 并集合并；第三条无 scope
const MY_PERMS: {
  permissions: ProfilePermission[];
  permissionIDs: string[];
  rolePermissions: Array<{ role: string; permissionIds: string[] }>;
  fullAccess: boolean;
} = {
  permissions: [
    { resource: 'pages', actions: ['edit', 'publish'], gameId: 'demo', env: 'prod' },
    { resource: 'pages', actions: ['rollback'], gameId: 'demo', env: 'prod' },
    { resource: 'audit', actions: ['read'], gameId: '', env: '' },
  ],
  permissionIDs: ['ops:manage'],
  rolePermissions: [{ role: 'admin', permissionIds: ['pages:edit', 'audit:read'] }],
  fullAccess: false,
};

const ACTIVITIES: AuditEvent[] = [
  {
    id: 'audit_1',
    time: '2026-09-01T10:00:00Z',
    kind: 'page.publish',
    actor: 'alice',
    target: 'home',
    meta: {},
    hash: 'h1',
    prev: '',
  },
  {
    id: 'audit_2',
    time: '2026-09-02T11:00:00Z',
    kind: 'function.invoke',
    actor: 'alice',
    target: 'fn.echo',
    meta: {},
    hash: 'h2',
    prev: '',
  },
];

// ip 键族矩阵（ip / client_ip）+ 成败两翼 + time 空串（key 落 idx 兜底）
const LOGIN_EVENTS: AuditEvent[] = [
  {
    id: 'l1',
    time: '2026-09-01T10:00:00Z',
    kind: 'auth_login',
    actor: 'alice',
    target: 'alice',
    meta: { ip: '1.2.3.4', ipRegion: '上海', userAgent: 'Mozilla/5.0' },
    hash: 'h1',
    prev: '',
  },
  {
    id: 'l2',
    time: '2026-09-02T12:00:00Z',
    kind: 'login_fail',
    actor: 'alice',
    target: 'alice',
    meta: { client_ip: '5.6.7.8', region: '北京', ua: 'curl/8.0' },
    hash: '',
    prev: '',
  },
  {
    id: 'l3',
    time: '',
    kind: 'login_rate_limited',
    actor: 'alice',
    target: 'alice',
    meta: {},
    hash: '',
    prev: '',
  },
  // kind 空串是服务端真实形态（normalizeAuditEvent 的 action ?? ''）——
  // 成败推断按「不含 fail/rate_limited 即成功」兜底
  {
    id: 'l4',
    time: '2026-09-03T09:00:00Z',
    kind: '',
    actor: 'alice',
    target: 'alice',
    meta: {},
    hash: '',
    prev: '',
  },
];

const MESSAGES: MessageItem[] = [
  {
    id: 'm1',
    to: 'alice',
    type: 'system',
    title: '维护通知',
    content: '今晚维护',
    data: { approvalId: 5 },
    status: 'unread',
    createdAt: '2026-09-03T00:00:00Z',
    updatedAt: '2026-09-03T00:00:00Z',
  },
  {
    id: 'm2',
    to: 'alice',
    type: 'approval',
    title: '审批提醒',
    content: '请审批',
    data: null,
    status: 'read',
    readAt: '2026-09-04T00:00:00Z',
    createdAt: '2026-09-04T00:00:00Z',
    updatedAt: '2026-09-04T00:00:00Z',
  },
];

const mkPerm = (
  id: string,
  name: string,
  resource: string,
  action: string,
  category: string,
): PermissionRecord => ({
  id,
  name,
  description: `${name}的描述`,
  resource,
  action,
  category,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
});

// pages:edit 被 key 过滤（owned）、ops:manage 被 id 过滤（permissionIDs）、
// functions:manage 存活、billing:read name 空串（目录树名回退 id）→ 候选两条
const CATALOG: PermissionRecord[] = [
  mkPerm('pages:edit', '页面编辑', 'pages', 'edit', 'page'),
  mkPerm('functions:manage', '函数管理', 'functions', 'manage', 'functions'),
  mkPerm('ops:manage', '运维管理', 'ops', 'manage', 'ops'),
  mkPerm('billing:read', '', 'billing', 'read', 'billing'),
];

const CHANNELS = [
  { key: 'in_app', available: true, userEnabled: true },
  {
    key: 'email',
    available: false,
    userEnabled: false,
    reason: 'smtp_missing',
    requiresTarget: true,
    hasTarget: false,
  },
];

const LOGIN_KINDS = 'login,auth_login,login_fail,login_rate_limited';

function renderPage() {
  return render(
    <App>
      <ProfilePage />
    </App>,
  );
}

async function waitHero() {
  expect(await screen.findByText('Alice Admin')).toBeInTheDocument();
  await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));
}

/** stats 卡取值（title → .ant-statistic-content 文本） */
function statValue(title: string): string {
  const card = screen.getByText(title).closest('.ant-statistic');
  expect(card).not.toBeNull();
  return (card?.querySelector('.ant-statistic-content') as HTMLElement)?.textContent ?? '';
}

/** InfoTab 卡（查看态「账户信息」/ 编辑态「资料设置」标题所在卡） */
function infoCard(): HTMLElement {
  const el = screen.queryByText('账户信息') ?? screen.getByText('资料设置');
  return (el as HTMLElement).closest('.ant-card') as HTMLElement;
}

/** 弹窗关闭态：非 destroyOnClose 的 antd Modal 关闭后壳以 display:none 残留
 * （Round-15 坑档），沿祖先链查 display:none——挂在 wrap 还是 root 均覆盖 */
function modalWrapHidden(): boolean {
  const modal = document.querySelector('.ant-modal');
  if (modal === null) return true;
  let node: HTMLElement | null = modal as HTMLElement;
  while (node && node !== document.body) {
    if (node.style?.display === 'none') return true;
    node = node.parentElement;
  }
  return false;
}

/** 按卡片头文本找 Card（「权限概览」与 Tab 标签同文本，getByText 双命中，
 * 卡片锚定绕开 tab） */
function cardByTitle(title: string): HTMLElement {
  const card = Array.from(document.querySelectorAll('.ant-card')).find((c) =>
    c.querySelector('.ant-card-head')?.textContent?.includes(title),
  );
  expect(card).toBeTruthy();
  return card as HTMLElement;
}

/** 登录记录表（页面内唯一 .ant-table——InfoTab/Descriptions 无表格；
 * 已激活 pane 缓存不卸载，跨 pane 同文本断言一律收窄到本表） */
function sessionsTable(): HTMLElement {
  const tables = document.querySelectorAll('.ant-table');
  expect(tables.length).toBeGreaterThan(0);
  return tables[tables.length - 1] as HTMLElement;
}

/** 进入编辑态：渲染页面 → 等 hero → 点 InfoTab 卡 extra 的「编辑资料」按钮
 *（hero 同名按钮在 DOM 前面，within(card) 收窄） */
async function enterEdit() {
  renderPage();
  await waitHero();
  fireEvent.click(within(infoCard()).getByRole('button', { name: /编\s*辑/ }));
  await waitFor(() => expect(screen.getByText('资料设置')).toBeInTheDocument());
}

beforeEach(() => {
  jest.clearAllMocks();
  mockSearch = '';
  mProfile.mockResolvedValue(PROFILE);
  mGames.mockResolvedValue({ games: GAMES });
  mPerms.mockResolvedValue(MY_PERMS);
  mAudit.mockImplementation(async (params?: { kinds?: string }) =>
    params?.kinds ? { events: LOGIN_EVENTS } : { events: ACTIVITIES },
  );
  mMessages.mockResolvedValue({ items: MESSAGES, total: 2, page: 1, pageSize: 8 });
  mMarkRead.mockResolvedValue(undefined);
  mCatalog.mockResolvedValue({ items: CATALOG, total: 3, page: 1, pageSize: 500 });
  mChannels.mockResolvedValue({ channels: CHANNELS });
  mMfaStatus.mockResolvedValue({
    enabled: false,
    local: true,
    recoveryCodesRemaining: 0,
    recoveryCodeTotal: 10,
  });
  mFeedback.mockResolvedValue({} as never);
  mUpdate.mockResolvedValue({ ...PROFILE });
});

describe('个人中心 首拉链与 hero', () => {
  it('首拉七路并行 + hero 字段矩阵 + stats 四卡 + InfoTab 默认面板（含最近登录IP）', async () => {
    renderPage();
    await waitHero();

    // 七路并行调用参数
    expect(mProfile).toHaveBeenCalledTimes(1);
    expect(mPerms).toHaveBeenCalledWith({});
    expect(mAudit).toHaveBeenCalledWith({ actor: 'alice', size: 8 });
    expect(mAudit).toHaveBeenCalledWith({ actor: 'alice', kinds: LOGIN_KINDS, size: 20 });
    expect(mMessages).toHaveBeenCalledWith({ status: 'all', pageSize: 8 });
    expect(mCatalog).toHaveBeenCalledWith({ page: 1, pageSize: 500 });
    expect(mChannels).toHaveBeenCalledTimes(1);

    // hero：启用徽标 / 角色 Tag / Descriptions 格式化
    expect(screen.getByText('启用')).toBeInTheDocument();
    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('ops')).toBeInTheDocument();
    expect(screen.getAllByText('alice@example.com').length).toBeGreaterThan(0);
    expect(screen.getAllByText('13800138000').length).toBeGreaterThan(0);
    expect(screen.getAllByText(formatDateTime('2026-01-01T00:00:00Z')).length).toBeGreaterThan(0);
    expect(screen.getAllByText(formatDateTime('2026-09-01T00:00:00Z')).length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: /更换头像/ })).toBeInTheDocument();

    // stats 四卡派生值（授权游戏=GAMES 3 项）
    expect(statValue('授权游戏')).toBe('3');
    expect(statValue('角色')).toBe('2');
    expect(statValue('权限项')).toBe('3');
    expect(statValue('近期操作')).toBe('2');

    // InfoTab 默认挂载：七行 Descriptions + 最近登录IP 取首个有 IP 行
    const card = infoCard();
    expect(within(card).getByText('42')).toBeInTheDocument();
    expect(within(card).getAllByText('alice').length).toBeGreaterThan(0);
    expect(within(card).getByText('1.2.3.4')).toBeInTheDocument();
  });

  it('加载分支与失败分支：pending 骨架、reject toast 且停留加载态', async () => {
    mProfile.mockReturnValueOnce(new Promise(() => {}));
    const { unmount } = renderPage();
    expect(await screen.findByText('正在加载个人信息...')).toBeInTheDocument();
    expect(document.querySelector('.ant-avatar')).not.toBeNull();
    expect(screen.queryByText('Alice Admin')).not.toBeInTheDocument();
    unmount();

    mProfile.mockRejectedValueOnce(new Error('profile down'));
    renderPage();
    expect(await screen.findByText('加载个人信息失败')).toBeInTheDocument();
    // profile 保持 null：hero 不渲染，骨架仍在
    expect(screen.getByText('正在加载个人信息...')).toBeInTheDocument();
    expect(mGames).not.toHaveBeenCalled();
  });

  it('未设置兜底矩阵：空 displayName/邮箱/手机/时间 + roles undefined + active 未定义/显式 false', async () => {
    mProfile.mockResolvedValueOnce({
      ...PROFILE,
      username: 'bob',
      nickname: 'Bobby',
      displayName: undefined,
      email: '',
      phone: '',
      createdAt: '',
      lastLoginAt: '',
      active: undefined,
      // roles 显式 undefined（spread 会带上 PROFILE 的 roles）：右翼覆盖
      roles: undefined,
    });
    const { unmount } = renderPage();

    // Title 回退 username（hero/头像占位/InfoTab 三处）；无布尔 active → 无徽标
    expect((await screen.findAllByText('bob')).length).toBeGreaterThan(0);
    expect(screen.queryByText('启用')).not.toBeInTheDocument();
    expect(screen.queryByText('未启用')).not.toBeInTheDocument();
    // hero 四行未设置（邮箱/手机/加入/最后登录）
    expect(screen.getAllByText('未设置').length).toBeGreaterThanOrEqual(4);
    // InfoTab 行未设置（邮箱/手机/加入/最后登录）
    const card = infoCard();
    expect(within(card).getAllByText('未设置').length).toBeGreaterThanOrEqual(4);
    unmount();

    // active 显式 false：徽标走 default/未启用 翼
    mProfile.mockResolvedValueOnce({
      ...PROFILE,
      username: 'bob',
      nickname: 'Bobby',
      displayName: undefined,
      active: false,
    });
    renderPage();
    expect(await screen.findByText('未启用')).toBeInTheDocument();
    expect(screen.queryByText('启用')).not.toBeInTheDocument();
  });
});

describe('个人中心 Tab 编排与 URL 同步', () => {
  it('切到安全中心：pane 真实渲染（密码入口/通道状态/有记录/MFA 状态拉取）+ navigate replace', async () => {
    renderPage();
    await waitHero();

    fireEvent.click(screen.getByRole('tab', { name: /安全中心/ }));
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith('/profile?tab=security', { replace: true }),
    );
    await waitFor(() => expect(screen.getByText('通知渠道偏好')).toBeInTheDocument());

    expect(screen.getByRole('button', { name: '修改密码' })).toBeInTheDocument();
    expect(screen.getByText('有记录')).toBeInTheDocument();
    expect(screen.getByText('已开启')).toBeInTheDocument();
    expect(screen.getByText('未接入')).toBeInTheDocument();
    // 通道开关只读呈现 userEnabled（disabled，BUG-016）：接入+开启 → 勾选；
    // 未接入 → 不勾选
    expect(screen.getByTestId('channel-switch-email')).toBeDisabled();
    expect(screen.getByTestId('channel-switch-in_app')).toBeDisabled();
    expect(screen.getByTestId('channel-switch-in_app')).toBeChecked();
    expect(screen.getByTestId('channel-switch-email')).not.toBeChecked();
    expect(mMfaStatus).toHaveBeenCalledTimes(1);
  });

  it('深链 ?tab=games：GamesTab 首挂载、InfoTab 不挂载；切回资料后共存（pane 缓存）', async () => {
    mockSearch = '?tab=games';
    renderPage();

    expect(await screen.findByText('Demo Game')).toBeInTheDocument();
    expect(screen.queryByText('账户信息')).not.toBeInTheDocument();

    // 三形态权限区（BUG-018 矩阵）+ 环境Tag
    expect(screen.getByTestId('game-permissions-demo').getAttribute('data-access')).toBe('scoped');
    expect(screen.getByTestId('game-access-full-ops')).toBeInTheDocument();
    expect(screen.getByTestId('game-access-none-empty')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('tab', { name: /个人信息/ }));
    expect(await screen.findByText('账户信息')).toBeInTheDocument();
    expect(screen.getByText('Demo Game')).toBeInTheDocument();
  });

  it('会话/活动 Tab：登录记录六列矩阵（成败/多键IP/属地/UA/类型）+ 活动审计 + 刷新重拉', async () => {
    renderPage();
    await waitHero();

    fireEvent.click(screen.getByRole('tab', { name: /登录记录/ }));
    await waitFor(() => expect(within(sessionsTable()).getByText('1.2.3.4')).toBeInTheDocument());

    const table = sessionsTable();
    // 成败从 kind 推断：auth_login 与 kind 空串（服务端 action ?? '' 形态）
    // 均成功；login_fail/login_rate_limited 失败
    expect(within(table).getAllByText('成功')).toHaveLength(2);
    expect(within(table).getAllByText('失败')).toHaveLength(2);
    // ip 键族：ip / client_ip；region 键族：ipRegion / region；ua 键族：userAgent / ua
    expect(within(table).getByText('5.6.7.8')).toBeInTheDocument();
    expect(within(table).getByText('上海')).toBeInTheDocument();
    expect(within(table).getByText('北京')).toBeInTheDocument();
    expect(within(table).getByText('Mozilla/5.0')).toBeInTheDocument();
    expect(within(table).getByText('curl/8.0')).toBeInTheDocument();
    expect(within(table).getByText('auth_login')).toBeInTheDocument();
    expect(within(table).getByText('login_rate_limited')).toBeInTheDocument();
    // time 空串 → formatDateTime('') = '-'
    expect(within(table).getAllByText('-').length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole('tab', { name: /最近操作/ }));
    expect(await screen.findByText('page.publish')).toBeInTheDocument();
    expect(screen.getByText('function.invoke')).toBeInTheDocument();
    expect(screen.getAllByText(formatDateTime('2026-09-01T10:00:00Z')).length).toBeGreaterThan(0);

    // 刷新按钮 → loadExtras(username) 再拉七路（锚其中两路计数）
    fireEvent.click(screen.getAllByRole('button', { name: /重新加载/ })[0]);
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(2));
    expect(mAudit).toHaveBeenCalledTimes(4);
  });

  it('hero 编辑按钮：切回资料 Tab 进编辑态 + navigate ?tab=profile + scrollIntoView', async () => {
    mockSearch = '?tab=notifications';
    renderPage();
    await waitHero();

    // hero 的「编辑资料」在 DOM 前面（InfoTab 未挂载时全页唯一）
    fireEvent.click(screen.getByRole('button', { name: /编辑资料/ }));
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith('/profile?tab=profile', { replace: true }),
    );
    // 编辑表单挂载且回填 profile 值
    const input = await screen.findByPlaceholderText('请输入显示名称');
    expect((input as HTMLInputElement).value).toBe('Alice Admin');
    expect(screen.getByText('资料设置')).toBeInTheDocument();
    await waitFor(() => expect(scrollIntoViewMock).toHaveBeenCalled());
  });
});

describe('个人中心 资料编辑链', () => {
  it('保存成功链：updateMyProfile 载荷 + toast + 退出编辑 + 重拉资料', async () => {
    await enterEdit();

    fireEvent.change(screen.getByPlaceholderText('请输入显示名称'), {
      target: { value: 'Alice New' },
    });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));

    await waitFor(() =>
      expect(mUpdate).toHaveBeenCalledWith({
        displayName: 'Alice New',
        email: 'alice@example.com',
        phone: '13800138000',
      }),
    );
    expect(await screen.findByText('个人信息更新成功')).toBeInTheDocument();
    // 退出编辑态 + 重拉
    await waitFor(() => expect(screen.getByText('账户信息')).toBeInTheDocument());
    await waitFor(() => expect(mProfile).toHaveBeenCalledTimes(2));
  });

  it('保存失败翼 + 校验拦截：error toast 保持编辑态；displayName 必填 / phone pattern', async () => {
    await enterEdit();

    // 失败翼
    mUpdate.mockRejectedValueOnce(new Error('update down'));
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(await screen.findByText('更新个人信息失败')).toBeInTheDocument();
    expect(screen.getByText('资料设置')).toBeInTheDocument();

    // 必填拦截
    fireEvent.change(screen.getByPlaceholderText('请输入显示名称'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(await screen.findByText('显示名称不能为空')).toBeInTheDocument();

    // phone pattern（displayName 已恢复合法值）
    fireEvent.change(screen.getByPlaceholderText('请输入显示名称'), {
      target: { value: 'Alice New' },
    });
    fireEvent.change(screen.getByPlaceholderText('请输入手机号'), { target: { value: '123' } });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(await screen.findByText('请输入合法手机号')).toBeInTheDocument();
    expect(mUpdate).toHaveBeenCalledTimes(1);
  });

  it('取消编辑：表单回填 profile 值并退回查看态', async () => {
    await enterEdit();

    fireEvent.change(screen.getByPlaceholderText('请输入显示名称'), {
      target: { value: 'Junk' },
    });
    fireEvent.click(screen.getByRole('button', { name: /取消编辑/ }));
    await waitFor(() => expect(screen.getByText('账户信息')).toBeInTheDocument());

    // 再进编辑：值已被 onCancelEdit 的 setFieldsValue 复位
    fireEvent.click(within(infoCard()).getByRole('button', { name: /编\s*辑/ }));
    await waitFor(() =>
      expect((screen.getByPlaceholderText('请输入显示名称') as HTMLInputElement).value).toBe(
        'Alice Admin',
      ),
    );
  });

  it('取消回填右翼：displayName 空串时 onCancelEdit 回填 nickname', async () => {
    mProfile.mockResolvedValueOnce({ ...PROFILE, displayName: '' });
    mockSearch = '?tab=notifications';
    renderPage();
    // displayName 空串：hero Title 回退 username（191 行链），nickname 只进
    // 取消回填与头像占位
    expect((await screen.findAllByText('alice')).length).toBeGreaterThan(0);
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: /编辑资料/ }));
    expect(await screen.findByText('资料设置')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /取消编辑/ }));
    await waitFor(() => expect(screen.getByText('账户信息')).toBeInTheDocument());

    // 再进编辑：displayName || nickname 右翼——表单值取 nickname 'Ali'
    fireEvent.click(within(infoCard()).getByRole('button', { name: /编\s*辑/ }));
    await waitFor(() =>
      expect((screen.getByPlaceholderText('请输入显示名称') as HTMLInputElement).value).toBe('Ali'),
    );
  });
});

describe('个人中心 消息已读链（openMessage / markMessageRead / markAllRead）', () => {
  async function openNotifications() {
    renderPage();
    await waitHero();
    fireEvent.click(screen.getByRole('tab', { name: /消息通知/ }));
    await waitFor(() => expect(screen.getByTestId('notification-m1')).toBeInTheDocument());
  }

  it('未读徽标 + 行渲染（含数据/审批外链）+ 全部标为已读按 unread 过滤', async () => {
    await openNotifications();

    expect(screen.getByTestId('notifications-unread-count')).toBeInTheDocument();
    expect(screen.getByText('维护通知')).toBeInTheDocument();
    expect(screen.getByText('审批提醒')).toBeInTheDocument();
    expect(screen.getByText('含数据')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /查看审批/ }).getAttribute('href')).toBe(
      '/approvals?approvalId=5',
    );

    fireEvent.click(screen.getByTestId('notifications-mark-all-read'));
    await waitFor(() => expect(mMarkRead).toHaveBeenCalledWith(['m1']));
    // 徽标与按钮随 unreadCount 归零消失；行翻已读
    await waitFor(() =>
      expect(screen.queryByTestId('notifications-unread-count')).not.toBeInTheDocument(),
    );
    expect(screen.queryByTestId('notifications-mark-all-read')).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByTestId('notification-m1').getAttribute('data-unread')).toBe('false'),
    );
  });

  it('单条标为已读：markMessagesRead([id]) + 状态翻转，不弹详情', async () => {
    await openNotifications();

    fireEvent.click(screen.getByTestId('notification-mark-read-m1'));
    await waitFor(() => expect(mMarkRead).toHaveBeenCalledWith(['m1']));
    await waitFor(() =>
      expect(screen.getByTestId('notification-m1').getAttribute('data-unread')).toBe('false'),
    );
    // 未弹详情（stopPropagation 语义）：详情 Modal 从未挂载
    expect(document.querySelector('.ant-modal')).toBeNull();
  });

  it('详情链：打开未读消息即标读并翻转详情状态；已读消息打开不触发标读', async () => {
    await openNotifications();
    mMarkRead.mockClear();

    // 标题锚定 .ant-modal-title——列表行与详情弹窗同文本（请审批）双命中
    const modalTitle = () => document.querySelector('.ant-modal-title')?.textContent ?? '';

    // 已读消息：开详情不触发标读（弹窗标题取 m2.title）
    fireEvent.click(screen.getByTestId('notification-m2'));
    await waitFor(() => expect(modalTitle()).toBe('审批提醒'));
    expect(mMarkRead).not.toHaveBeenCalled();

    // 关闭后开未读消息：弹窗内容切换（关闭可复用的可观测后果）
    fireEvent.click(document.querySelector('.ant-modal-close') as HTMLElement);
    fireEvent.click(screen.getByTestId('notification-m1'));
    await waitFor(() => expect(modalTitle()).toBe('维护通知'));
    await waitFor(() => expect(mMarkRead).toHaveBeenCalledWith(['m1']));

    // 未读消息：开详情即标读，详情内状态从「未读」翻「已读」
    const modal = document.querySelector('.ant-modal') as HTMLElement;
    await waitFor(() => expect(within(modal).getByText('已读')).toBeInTheDocument());
    // data 审批载荷以 JSON 形态展示
    expect(modal.querySelector('pre')?.textContent).toContain('approvalId');
    await waitFor(() =>
      expect(screen.getByTestId('notification-m1').getAttribute('data-unread')).toBe('false'),
    );
  });
});

describe('个人中心 权限派生与申请链', () => {
  async function openPermissions() {
    renderPage();
    await waitHero();
    fireEvent.click(screen.getByRole('tab', { name: /权限概览/ }));
    await waitFor(() => expect(screen.getByText('可申请权限')).toBeInTheDocument());
  }

  it('权限 Tab：分组并集合并 + scope Tag + 目录驱动的候选双过滤 + 权限树角色', async () => {
    await openPermissions();

    // 概览卡：pages 两条合并为一组（edit/publish/rollback 并集）+ scope Tag
    const summaryCard = cardByTitle('权限概览');
    expect(within(summaryCard).getAllByText('pages')).toHaveLength(1);
    expect(within(summaryCard).getByText('demo prod')).toBeInTheDocument();
    expect(within(summaryCard).getByText('edit')).toBeInTheDocument();
    expect(within(summaryCard).getByText('publish')).toBeInTheDocument();
    expect(within(summaryCard).getByText('rollback')).toBeInTheDocument();

    // 申请卡：目录可用绿标；候选 = functions:manage（key/id 双过滤后存活）+
    // billing:read（name 空串，行标题空、resource:action Tag 正常）。
    // 函数管理 双命中于权限树目录（未拥有灰字）——断言收窄到申请卡
    expect(screen.getByText('基于系统权限目录')).toBeInTheDocument();
    const applyCard = cardByTitle('可申请权限');
    expect(within(applyCard).getByText('函数管理')).toBeInTheDocument();
    expect(within(applyCard).getByText('functions:manage')).toBeInTheDocument();
    expect(within(applyCard).getByText('billing:read')).toBeInTheDocument();
    expect(within(applyCard).queryByText('页面编辑')).not.toBeInTheDocument();
    expect(within(applyCard).queryByText('运维管理')).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: '申请' })).toHaveLength(2);

    // 权限树：角色名渲染（hero 的 admin Tag 之外的第二处）
    expect(screen.getAllByText('admin').length).toBeGreaterThanOrEqual(2);
    // fullAccess=false → 无全量徽标
    expect(screen.queryByTestId('perm-tree-full-access')).not.toBeInTheDocument();
  });

  it('目录空列表：fallback 模板兜底（经已拥有过滤后仅 functions:manage 存活），标记仍为绿（catalogAvailable 只认请求成败）', async () => {
    mCatalog.mockResolvedValueOnce({ items: [], total: 0, page: 1, pageSize: 500 });
    await openPermissions();

    // 现状行为锁定：catalogAvailable 仅在 listPermissions reject 时翻 false——
    // 空列表 fulfilled 仍绿标，但候选已按 catalog.length 回退六模板。
    // 语义若日后收紧（空列表也标受限），本用例同步翻转。
    // owned 集合 = permissions(resource:action) ∪ permissionIds：夹具拥有
    // pages:edit/publish/rollback、audit:read、ops:manage，六模板仅
    // functions:manage 存活
    expect(screen.getByText('基于系统权限目录')).toBeInTheDocument();
    const applyCard = cardByTitle('可申请权限');
    expect(within(applyCard).getByText('函数管理权限')).toBeInTheDocument();
    expect(within(applyCard).queryByText('页面编辑权限')).not.toBeInTheDocument();
    expect(within(applyCard).queryByText('页面发布权限')).not.toBeInTheDocument();
    expect(within(applyCard).queryByText('审计读取权限')).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: '申请' })).toHaveLength(1);
  });

  it('申请弹窗：reason 必填拦截 + createFeedback 载荷 + 成功 toast + 关闭', async () => {
    await openPermissions();

    // 两条候选各带一个申请按钮，取首个（functions:manage）
    fireEvent.click(screen.getAllByRole('button', { name: '申请' })[0]);
    await waitFor(() =>
      expect(
        Array.from(document.querySelectorAll('.ant-modal-title')).some(
          (t) => t.textContent === '申请权限',
        ),
      ).toBe(true),
    );

    // 空理由提交：required 拦截、不触达服务
    fireEvent.click(screen.getByRole('button', { name: '提交申请' }));
    expect(await screen.findByText('请填写申请理由')).toBeInTheDocument();
    expect(mFeedback).not.toHaveBeenCalled();

    fireEvent.change(screen.getByPlaceholderText('请说明业务场景、影响范围、预计使用时长等信息'), {
      target: { value: '需要函数管理权限' },
    });
    fireEvent.click(screen.getByRole('button', { name: '提交申请' }));

    await waitFor(() =>
      expect(mFeedback).toHaveBeenCalledWith(
        expect.objectContaining({
          category: 'permission_request',
          priority: 'normal',
          source: 'profile_permission_apply',
          content: expect.stringContaining('权限标识: functions:manage'),
        }),
      ),
    );
    expect(await screen.findByText('权限申请已提交')).toBeInTheDocument();
    // destroyOnHidden：弹窗整体卸载
    await waitFor(() =>
      expect(
        screen.queryByPlaceholderText('请说明业务场景、影响范围、预计使用时长等信息'),
      ).toBeNull(),
    );
  });

  it('fullAccess=true：权限树全量徽标渲染、页面不炸', async () => {
    mPerms.mockResolvedValueOnce({ ...MY_PERMS, fullAccess: true });
    await openPermissions();

    expect(screen.getByTestId('perm-tree-full-access')).toBeInTheDocument();
    expect(cardByTitle('权限概览')).toBeInTheDocument();
  });

  it('env-only 权限行：gameId 空而 env 有值 → scope 取 env 单值 Tag', async () => {
    mPerms.mockResolvedValueOnce({
      permissions: [{ resource: 'billing', actions: ['read'], gameId: '', env: 'prod' }],
      permissionIDs: [],
      rolePermissions: [],
      fullAccess: false,
    });
    await openPermissions();

    const summaryCard = cardByTitle('权限概览');
    expect(within(summaryCard).getByText('billing')).toBeInTheDocument();
    expect(within(summaryCard).getByText('prod')).toBeInTheDocument();
    expect(statValue('权限项')).toBe('1');
  });
});

describe('个人中心 loadExtras 失败矩阵与兜底', () => {
  it('七路全 reject：settled 静默（无 extras.error toast）、stats 归零、各 Tab 空态、目录回退', async () => {
    mGames.mockRejectedValueOnce(new Error('games down'));
    mPerms.mockRejectedValueOnce(new Error('perms down'));
    // listAudit 每轮 loadExtras 发两次（activities + login），两个 Once 都要排
    mAudit.mockRejectedValueOnce(new Error('audit down'));
    mAudit.mockRejectedValueOnce(new Error('login audit down'));
    mMessages.mockRejectedValueOnce(new Error('messages down'));
    mCatalog.mockRejectedValueOnce(new Error('catalog down'));
    mChannels.mockRejectedValueOnce(new Error('channels down'));
    renderPage();

    await waitHero();
    // allSettled 静默：外层 catch 不触发
    await waitFor(() => expect(mChannels).toHaveBeenCalledTimes(1));
    expect(screen.queryByText('加载账号洞察失败')).not.toBeInTheDocument();

    // stats 归零（games/perms 双 reject；activities reject）
    await waitFor(() => expect(statValue('权限项')).toBe('0'));
    expect(statValue('授权游戏')).toBe('0');
    expect(statValue('近期操作')).toBe('0');

    // 各 Tab 空态 + 目录受限回退
    fireEvent.click(screen.getByRole('tab', { name: /权限概览/ }));
    expect(await screen.findByText('权限目录受限，展示推荐清单')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: /登录记录/ }));
    expect(
      await screen.findByText(
        '当前账号还没有可展示的登录审计记录，只有真实登录事件才会出现在这里。',
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: /最近操作/ }));
    expect(
      await screen.findByText('当前账号还没有可展示的最近操作记录，页面不再伪造示例数据。'),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: /消息通知/ }));
    expect(
      await screen.findByText('当前账号没有站内消息，消息中心仅展示真实消息数据。'),
    ).toBeInTheDocument();
  });

  it('perms 载荷 nullish 右翼（resolve {}）+ username 空串：login 查询不发、会话空态', async () => {
    mPerms.mockResolvedValueOnce({} as never);
    mProfile.mockResolvedValueOnce({ ...PROFILE, username: '' });
    renderPage();
    await waitHero();

    await waitFor(() => expect(statValue('权限项')).toBe('0'));
    // username 空串：kinds 查询走 Promise.resolve 右翼，listAudit 只发 activities 一次
    expect(mAudit).toHaveBeenCalledTimes(1);
    expect(mAudit).toHaveBeenCalledWith({ actor: '', size: 8 });

    fireEvent.click(screen.getByRole('tab', { name: /登录记录/ }));
    expect(
      await screen.findByText(
        '当前账号还没有可展示的登录审计记录，只有真实登录事件才会出现在这里。',
      ),
    ).toBeInTheDocument();
    // 目录仍可用（catalog 成功）：绿色标记
    fireEvent.click(screen.getByRole('tab', { name: /权限概览/ }));
    expect(await screen.findByText('基于系统权限目录')).toBeInTheDocument();
  });
});

describe('个人中心 密码/头像弹窗接线', () => {
  it('PasswordModal：安全页「修改密码」入口开合，不触达改密服务', async () => {
    renderPage();
    await waitHero();
    fireEvent.click(screen.getByRole('tab', { name: /安全中心/ }));
    await waitFor(() => expect(screen.getByRole('button', { name: '修改密码' })).toBeEnabled());

    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));
    expect(await screen.findByPlaceholderText('请输入当前密码')).toBeInTheDocument();
    expect(
      Array.from(document.querySelectorAll('.ant-modal-title')).some(
        (t) => t.textContent === '修改密码',
      ),
    ).toBe(true);

    fireEvent.click(document.querySelector('.ant-modal-close') as HTMLElement);
    await waitFor(() => expect(modalWrapHidden()).toBe(true));
    // 可重开（关闭接线的可观测后果）
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));
    expect(await screen.findByPlaceholderText('请输入当前密码')).toBeInTheDocument();
    expect(mChangePwd).not.toHaveBeenCalled();
  });

  it('AvatarModal：hero「更换头像」入口开合（onPersisted 接线在弹窗自有套件域）', async () => {
    renderPage();
    await waitHero();

    fireEvent.click(screen.getByRole('button', { name: /更换头像/ }));
    expect(
      await screen.findByPlaceholderText('https://example.com/avatar.png'),
    ).toBeInTheDocument();
    expect(
      Array.from(document.querySelectorAll('.ant-modal-title')).some(
        (t) => t.textContent === '设置头像 URL',
      ),
    ).toBe(true);

    fireEvent.click(document.querySelector('.ant-modal-close') as HTMLElement);
    await waitFor(() => expect(modalWrapHidden()).toBe(true));
  });
});
