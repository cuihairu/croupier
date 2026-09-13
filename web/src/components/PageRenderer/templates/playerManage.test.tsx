/** playerManage 模板页覆盖。
 *
 * 覆盖路径：
 * - PageSpec 常量结构（列表列/行操作/批量操作/绑定形状）
 * - demo 执行器全分支：list（分页/筛选/非法分页参数兜底）、detail（命中/未命中/
 *   row 非对象）、create（全量/默认值兜底）、update（命中/未命中/字段缺省回退）、
 *   delete、action.ban、action.recharge（amount 缺省 0）、action.mail、
 *   action.batchBan/batchUnban（selection 数组化/非数组兜底）、未知 binding 兜底、
 *   requestId 递增
 * - 模板组件：PageContainer 标题（intl defaultMessage）、PageRenderer 收到
 *   pageSpec 与可用的 demo 执行器 */
import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { PageContainer } from '@ant-design/pro-components';
import PageRenderer from '@/components/PageRenderer';
import PlayerManageTemplate, {
  createPlayerManageDemoExecute,
  playerManagePageSpec,
} from './playerManage';
import type { BindingExecutionContext, JSONValue, PageExecutionResult } from '@/types/dashboard';

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({
    title,
    subTitle,
    children,
  }: {
    title?: React.ReactNode;
    subTitle?: React.ReactNode;
    children?: React.ReactNode;
  }) => (
    <div data-testid="page-container">
      <h1>{title}</h1>
      <p>{subTitle}</p>
      {children}
    </div>
  ),
}));

jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: jest.fn(
    (props: {
      pageSpec: { pageKey: string };
      onExecute?: (
        bindingId: string,
        context: BindingExecutionContext,
      ) => void | Promise<PageExecutionResult>;
    }) => <div data-testid="page-renderer" data-pagekey={props.pageSpec.pageKey} />,
  ),
}));

const mockedPageRenderer = PageRenderer as jest.MockedFunction<typeof PageRenderer>;

/** 列表返回的数据形状（demo 执行器内存实现） */
interface ListData {
  items: PlayerRow[];
  total: number;
}

interface PlayerRow {
  id: string;
  nickname: string;
  level: number;
  vip: number;
  status: 'active' | 'banned';
  balance: number;
}

type DemoExecute = ReturnType<typeof createPlayerManageDemoExecute>;

const ctx = (
  form?: JSONValue,
  row?: JSONValue,
  selection?: JSONValue,
): BindingExecutionContext => ({ form, row, selection });

const listData = async (execute: DemoExecute, form?: JSONValue): Promise<ListData> =>
  (await execute('list', ctx(form))).data as ListData;

const playerById = async (execute: DemoExecute, id: string): Promise<PlayerRow | null> =>
  (await execute('detail', ctx(undefined, { id }))).data as PlayerRow | null;

describe('PageSpec 常量', () => {
  it('resource 列表页形状：列、分页、行操作与批量操作', () => {
    expect(playerManagePageSpec.pageKey).toBe('template--player-manage');
    expect(playerManagePageSpec.type).toBe('resource');
    expect(playerManagePageSpec.resource?.listView.columns.map((c) => c.key)).toEqual([
      'id',
      'nickname',
      'level',
      'vip',
      'status',
      'balance',
      'lastLoginAt',
    ]);
    expect(playerManagePageSpec.resource?.listView.pagination).toEqual({
      enabled: true,
      defaultSize: 10,
      pageSizes: [10, 20, 50],
    });
    expect(playerManagePageSpec.resource?.listView.rowActions?.map((a) => a.key)).toEqual([
      'ban',
      'recharge',
      'mail',
      'edit',
    ]);
    expect(playerManagePageSpec.resource?.listView.batchActions?.map((a) => a.key)).toEqual([
      'batchBan',
      'batchUnban',
    ]);
  });

  it('绑定覆盖 CRUD 与行/批量动作', () => {
    expect(playerManagePageSpec.bindings.map((b) => b.id)).toEqual([
      'list',
      'detail',
      'create',
      'update',
      'delete',
      'action.ban',
      'action.recharge',
      'action.mail',
      'action.batchBan',
      'action.batchUnban',
    ]);
  });
});

describe('demo 执行器：list', () => {
  it('默认分页：第 1 页 10 条、总数 23、requestId 递增', async () => {
    const execute = createPlayerManageDemoExecute();
    const first = await execute('list', ctx());
    const second = await execute('list', ctx());
    const data = first.data as ListData;
    expect(data.items).toHaveLength(10);
    expect(data.total).toBe(23);
    expect(data.items[0]).toMatchObject({ id: '10001', nickname: '玩家1号', status: 'active' });
    expect(data.items[9].id).toBe('10010');
    expect(first.kind).toBe('sync');
    expect(first.requestId).toBe('demo-1');
    expect(second.requestId).toBe('demo-2');
  });

  it('指定 current/pageSize 翻页截取', async () => {
    const execute = createPlayerManageDemoExecute();
    const page3 = await listData(execute, { current: 3, pageSize: 10 });
    expect(page3.items.map((p) => p.id)).toEqual(['10021', '10022', '10023']);
    const page2 = await listData(execute, { current: 2, pageSize: 5 });
    expect(page2.items.map((p) => p.id)).toEqual(['10006', '10007', '10008', '10009', '10010']);
  });

  it('current/pageSize 非法时回退 1/10（含 form 为数组的兜底）', async () => {
    const execute = createPlayerManageDemoExecute();
    const bad = await listData(execute, { current: 'abc', pageSize: 'x' });
    expect(bad.items).toHaveLength(10);
    expect(bad.items[0].id).toBe('10001');
    const arrayForm = await listData(execute, [1, 2, 3]);
    expect(arrayForm.items).toHaveLength(10);
  });

  it('nickname/status 筛选与组合', async () => {
    const execute = createPlayerManageDemoExecute();
    const byNickname = await listData(execute, { nickname: '玩家2' });
    // 玩家2号、玩家20~23号
    expect(byNickname.items.map((p) => p.id)).toEqual([
      '10002',
      '10020',
      '10021',
      '10022',
      '10023',
    ]);

    const banned = await listData(execute, { status: 'banned' });
    expect(banned.items.map((p) => p.id)).toEqual(['10007', '10014', '10021']);

    const combined = await listData(execute, { nickname: '玩家2', status: 'banned' });
    expect(combined.items.map((p) => p.id)).toEqual(['10021']);
  });
});

describe('demo 执行器：detail', () => {
  it('命中玩家返回明细', async () => {
    const execute = createPlayerManageDemoExecute();
    const result = await execute('detail', ctx(undefined, { id: '10007' }));
    expect(result.data).toMatchObject({ id: '10007', status: 'banned', nickname: '玩家7号' });
  });

  it('未命中与 row 缺省/非对象都返回 null', async () => {
    const execute = createPlayerManageDemoExecute();
    expect((await execute('detail', ctx(undefined, { id: 'nope' }))).data).toBeNull();
    expect((await execute('detail', ctx(undefined, undefined))).data).toBeNull();
    expect((await execute('detail', ctx(undefined, ['array']))).data).toBeNull();
  });
});

describe('demo 执行器：create', () => {
  it('全量字段：新玩家插入队首，id 取最大值 +1', async () => {
    const execute = createPlayerManageDemoExecute();
    const created = await execute('create', ctx({ nickname: '新玩家', level: 10, vip: 3 }));
    expect(created.data).toEqual({ id: '10024' });
    const seeded = await listData(execute);
    expect(seeded.total).toBe(24);
    expect(seeded.items[0]).toMatchObject({
      id: '10024',
      nickname: '新玩家',
      level: 10,
      vip: 3,
      status: 'active',
      balance: 0,
    });
    expect(seeded.items[1].id).toBe('10001');
  });

  it('字段缺省：nickname 回退玩家{id}、level 0 回退 1、vip 回退 0', async () => {
    const execute = createPlayerManageDemoExecute();
    await execute('create', ctx());
    const withDefaults = await playerById(execute, '10024');
    expect(withDefaults).toMatchObject({ nickname: '玩家10024', level: 1, vip: 0 });

    const execute2 = createPlayerManageDemoExecute();
    await execute2('create', ctx({ nickname: '零级', level: 0, vip: 0 }));
    const zeroLevel = await playerById(execute2, '10024');
    expect(zeroLevel).toMatchObject({ nickname: '零级', level: 1, vip: 0 });
  });
});

describe('demo 执行器：update', () => {
  it('命中行更新，未命中行不受影响', async () => {
    const execute = createPlayerManageDemoExecute();
    const result = await execute(
      'update',
      ctx({ nickname: '改名', level: 99, vip: 8 }, { id: '10002' }),
    );
    expect(result.data).toEqual({ success: true });
    expect(await playerById(execute, '10002')).toMatchObject({
      nickname: '改名',
      level: 99,
      vip: 8,
    });
    expect(await playerById(execute, '10003')).toMatchObject({ nickname: '玩家3号', level: 26 });
    const miss = await execute('update', ctx({ nickname: '无效' }, { id: 'nope' }));
    expect(miss.data).toEqual({ success: true });
    expect(await playerById(execute, '10003')).toMatchObject({ nickname: '玩家3号' });
  });

  it('字段缺省回退原值', async () => {
    const execute = createPlayerManageDemoExecute();
    await execute('update', ctx(undefined, { id: '10005' }));
    expect(await playerById(execute, '10005')).toMatchObject({
      nickname: '玩家5号',
      level: 5 + ((5 * 7) % 55),
      vip: 5 % 6,
    });
  });
});

describe('demo 执行器：delete / 行动作', () => {
  it('delete 移除玩家', async () => {
    const execute = createPlayerManageDemoExecute();
    const result = await execute('delete', ctx(undefined, { id: '10001' }));
    expect(result.data).toEqual({ success: true });
    expect(await playerById(execute, '10001')).toBeNull();
    expect((await listData(execute)).total).toBe(22);
  });

  it('action.ban 命中行封禁，未命中行不变', async () => {
    const execute = createPlayerManageDemoExecute();
    const result = await execute('action.ban', ctx(undefined, { id: '10001' }));
    expect(result.data).toEqual({ success: true });
    expect((await playerById(execute, '10001'))?.status).toBe('banned');
    await execute('action.ban', ctx(undefined, { id: 'nope' }));
    expect((await playerById(execute, '10002'))?.status).toBe('active');
  });

  it('action.recharge 累加余额；amount 缺省按 0', async () => {
    const execute = createPlayerManageDemoExecute();
    const before = (await playerById(execute, '10001'))?.balance ?? 0;
    const result = await execute('action.recharge', ctx({ amount: 500 }, { id: '10001' }));
    expect(result.data).toEqual({ success: true, amount: 500 });
    expect((await playerById(execute, '10001'))?.balance).toBe(before + 500);

    const zero = await execute('action.recharge', ctx(undefined, { id: '10001' }));
    expect(zero.data).toEqual({ success: true, amount: 0 });
    expect((await playerById(execute, '10001'))?.balance).toBe(before + 500);
  });

  it('action.mail 返回成功', async () => {
    const execute = createPlayerManageDemoExecute();
    const result = await execute('action.mail', ctx({ title: 't', content: 'c' }, { id: '10001' }));
    expect(result.data).toEqual({ success: true });
  });
});

describe('demo 执行器：批量动作与兜底', () => {
  it('batchBan/batchUnban 按 selection 生效，selection 数字被 String 化', async () => {
    const execute = createPlayerManageDemoExecute();
    const ban = await execute('action.batchBan', ctx(undefined, undefined, [10001, 10002]));
    expect(ban.data).toEqual({ success: true, count: 2 });
    expect((await playerById(execute, '10001'))?.status).toBe('banned');
    expect((await playerById(execute, '10002'))?.status).toBe('banned');
    expect((await playerById(execute, '10003'))?.status).toBe('active');

    const unban = await execute('action.batchUnban', ctx(undefined, undefined, ['10001']));
    expect(unban.data).toEqual({ success: true, count: 1 });
    expect((await playerById(execute, '10001'))?.status).toBe('active');
    expect((await playerById(execute, '10002'))?.status).toBe('banned');
  });

  it('selection 非数组时按空集处理', async () => {
    const execute = createPlayerManageDemoExecute();
    const result = await execute('action.batchBan', ctx(undefined, undefined, undefined));
    expect(result.data).toEqual({ success: true, count: 0 });
    expect((await playerById(execute, '10001'))?.status).toBe('active');
  });

  it('未知 bindingId 兜底返回成功', async () => {
    const execute = createPlayerManageDemoExecute();
    const result = await execute('action.unknown', ctx());
    expect(result.data).toEqual({ success: true });
    expect(result.requestId).toBe('demo-1');
  });
});

describe('模板组件', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('渲染 PageContainer 标题与 PageRenderer（传入 pageSpec 与 demo 执行器）', async () => {
    render(<PlayerManageTemplate />);
    expect(screen.getByTestId('page-container')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '玩家管理' })).toBeInTheDocument();
    expect(screen.getByText('CRUD 模板 · 列表 / 详情 / 封禁 / 充值 / 邮件')).toBeInTheDocument();
    expect(screen.getByTestId('page-renderer')).toHaveAttribute(
      'data-pagekey',
      'template--player-manage',
    );

    expect(mockedPageRenderer).toHaveBeenCalledTimes(1);
    const props = mockedPageRenderer.mock.calls[0][0];
    expect(props.pageSpec).toBe(playerManagePageSpec);
    expect(typeof props.onExecute).toBe('function');

    // 透传的执行器即 demo 实现
    const result = await props.onExecute?.('list', ctx());
    const data = result?.data as ListData;
    expect(data.total).toBe(23);
    await waitFor(() => expect(screen.getByTestId('page-renderer')).toBeInTheDocument());
  });
});
