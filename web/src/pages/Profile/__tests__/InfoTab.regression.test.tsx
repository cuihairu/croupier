/**
 * 个人中心 InfoTab 回归（docs/BUGS.md BUG-007 / BUG-008）。
 *
 * 1) 头像 `src=""`：后端未设置头像时返回空串，React 把它渲染成
 *    `<img src="">`，浏览器会把空 src 当成「重新请求当前页」的 URL（并触发
 *    React 告警）。必须归一为 undefined，交给 icon 占位。
 * 2) 表单实例未连接：表单实例由主页 `useForm` 创建并复用（进入编辑前
 *    setFieldsValue 回填、保存时 submit）。此前 `<Form form={form}>` 只在编辑态
 *    渲染，未编辑时实例处于未连接状态，antd 每次渲染都告警
 *    "Instance created by `useForm` is not connected to any Form element"。
 *    现在表单常驻挂载、非编辑态用 `hidden` 隐藏。
 */
import React from 'react';
import fs from 'node:fs';
import path from 'node:path';
import { Form } from 'antd';
import type { FormInstance } from 'antd';
import { act, render, screen, waitFor } from '@testing-library/react';
import InfoTab from '../InfoTab';
import { normalizeAvatarSrc } from '../shared';
import type { ProfileData } from '../shared';

const noop = () => {};

function makeProfile(overrides: Partial<ProfileData> = {}): ProfileData {
  return {
    id: 7,
    username: 'admin',
    displayName: '系统管理员',
    email: 'admin@croupier.local',
    phone: '13800000000',
    ...overrides,
  } as ProfileData;
}

/** 收集 antd useForm 的「未连接」告警。 */
function collectFormConnectionWarnings(): string[] {
  const hits: string[] = [];
  const original = console.error;
  console.error = (...args: unknown[]) => {
    const text = args.map((a) => (a instanceof Error ? a.message : String(a))).join(' ');
    if (/not connected to any Form element/.test(text)) hits.push(text);
    original(...(args as []));
  };
  return {
    hits,
    restore: () => {
      console.error = original;
    },
  };
}

/**
 * 宿主组件：表单实例必须由组件内的 useForm 创建（不能在 render 之外调用
 * Form.useForm）。同时刻意让 form 实例在 InfoTab 之外创建，复现主页的真实拓扑。
 *
 * `attached=false` 时故意**不**渲染任何 `<Form form={form}>`，用于构造「孤立
 * 实例」对照组。
 *
 * 暴露 `fill` 以模拟 useProfileData 拉到资料后的 `form.setFieldsValue` 回填。
 */
let pendingForm: FormInstance | null = null;

function Host({
  editing,
  profile,
  attached = true,
}: {
  editing: boolean;
  profile: ProfileData;
  attached?: boolean;
}) {
  const [form] = Form.useForm();
  pendingForm = form;
  if (!attached) return <div data-testid="orphan-form-host" />;
  return (
    <InfoTab
      profile={profile}
      editing={editing}
      form={form}
      loading={false}
      latestLoginIP="127.0.0.1"
      onEdit={noop}
      onCancelEdit={noop}
      onSubmit={noop}
    />
  );
}

/** 对最近一次渲染的表单实例回填（等价于 useProfileData.ts:97 的行为）。 */
function pendingSetFieldsValue(): void {
  pendingForm?.setFieldsValue({
    displayName: '系统管理员',
    email: 'admin@croupier.local',
    phone: '13800000000',
  });
}

function renderInfoTab(editing: boolean, profile = makeProfile()) {
  return render(<Host editing={editing} profile={profile} />);
}

describe('InfoTab 表单常驻挂载（BUG-008）', () => {
  /**
   * 真实触发条件。
   *
   * antd 的 "not connected to any Form element" 告警不是挂载时报的，而是
   * `FormHook.warningUnhooked`——由**表单实例方法**（setFieldsValue /
   * getFieldValue / submit …）在 setTimeout 里检查 `formHooked` 触发。
   * 个人中心里 `useProfileData` 拉到资料后会 `form.setFieldsValue(...)` 回填
   * （useProfileData.ts:97），所以「未连接」+「调方法」才是线上复现路径；
   * 只挂载不调方法不会告警（本用例最初就漏在这里，变异测试没被抓住）。
   */
  it('未连接时调用 setFieldsValue 会触发 antd 告警（对照：证明告警通道有效）', async () => {
    const spy = collectFormConnectionWarnings();
    try {
      // attached=false：只有 useForm 出来的孤立实例，没有任何 <Form> 挂载它
      render(<Host editing={false} profile={makeProfile()} attached={false} />);
      await waitFor(() => {
        expect(screen.getByTestId('orphan-form-host')).toBeInTheDocument();
      });
      await act(async () => {
        pendingSetFieldsValue();
        await new Promise((r) => setTimeout(r, 20));
      });
    } finally {
      spy.restore();
    }
    // 该对照必须命中告警，否则下面的正向断言就是假阴性
    expect(spy.hits.length).toBeGreaterThan(0);
  });

  /**
   * 回归锁：表单在**两种状态**下都必须挂载。
   *
   * 为什么不用「断言没有告警」来锁？rc-util 的 `warning` 按 message 去重，
   * 同一条文案在一个模块生命周期内只报一次——上面的对照用例一旦先报过，
   * 后续任何「无告警」断言都恒真，无法区分修复与回归（已实测：把 InfoTab
   * 改回条件渲染，该断言仍然通过）。因此这里直接锁结构不变量：
   * `<Form>` 元素在非编辑态也必须在 DOM 中。
   */
  it('非编辑态隐藏表单但仍挂载，编辑态可见', () => {
    const { container } = renderInfoTab(false);
    const formEl = container.querySelector('form');
    expect(formEl).not.toBeNull();
    // hidden 容器：视觉上不出现，但 DOM 在（保证 form 实例已连接）
    expect(formEl?.closest('div[hidden]')).not.toBeNull();

    const { container: c2 } = renderInfoTab(true);
    const formEl2 = c2.querySelector('form');
    expect(formEl2).not.toBeNull();
    // 编辑态：容器不再带 hidden
    expect(formEl2?.closest('div[hidden]')).toBeNull();
  });

  it('非编辑态回填字段后实例已连接（不再产生未连接告警的新增实例）', async () => {
    renderInfoTab(false);
    await waitFor(() => {
      expect(screen.getAllByText('admin').length).toBeGreaterThan(0);
    });
    await act(async () => {
      pendingSetFieldsValue();
      await new Promise((r) => setTimeout(r, 20));
    });
    // 实例已连接 → 表单已挂载 → 字段值能落到真实 DOM 上
    expect(document.querySelector('form')).not.toBeNull();
  });

  /**
   * 回填归属 InfoTab 自身（BUG-008 的另一半修复）。
   *
   * 原先 `loadProfile` 在数据层无条件 `form.setFieldsValue`，但 `<Form>` 只存在于
   * Tabs「资料」面板内、antd 惰性渲染；用户停在 `?tab=security` 时实例未连接，
   * antd 每次渲染都告警。改为 InfoTab 挂载时自行回填。
   */
  it('资料到位后由 InfoTab 自行回填表单（非编辑态即已填好，进入编辑可直接改）', async () => {
    const { container } = renderInfoTab(false, makeProfile());
    await waitFor(() => {
      // 三个受控字段都应已带值
      const inputs = Array.from(container.querySelectorAll('input'));
      const values = inputs.map((i) => (i as HTMLInputElement).value);
      expect(values).toContain('系统管理员');
      expect(values).toContain('admin@croupier.local');
      expect(values).toContain('13800000000');
    });
  });

  it('回填随 profile 变化刷新（同值不重置，避免编辑中被覆盖）', async () => {
    const { container, rerender } = renderInfoTab(false, makeProfile());
    await waitFor(() => {
      expect(
        Array.from(container.querySelectorAll('input')).map((i) => (i as HTMLInputElement).value),
      ).toContain('系统管理员');
    });

    // 资料刷新为新值 → 表单跟着更新
    rerender(
      <Host
        editing={false}
        profile={makeProfile({ displayName: '新名字', email: 'new@croupier.local' })}
      />,
    );
    await waitFor(() => {
      const values = Array.from(container.querySelectorAll('input')).map(
        (i) => (i as HTMLInputElement).value,
      );
      expect(values).toContain('新名字');
      expect(values).toContain('new@croupier.local');
    });
  });

  it('非编辑态展示只读 Descriptions，编辑态展示可编辑表单项', () => {
    const { container: c1, unmount } = renderInfoTab(false);
    expect(c1.querySelector('.ant-descriptions')).not.toBeNull();
    unmount();

    const { container: c2 } = renderInfoTab(true);
    expect(c2.querySelector('.ant-descriptions')).toBeNull();
    // displayName / email / phone 三个受控表单项
    expect(c2.querySelectorAll('input').length).toBeGreaterThanOrEqual(3);
  });
});

describe('normalizeAvatarSrc（BUG-007）', () => {
  it('空串与空白串归一为 undefined', () => {
    expect(normalizeAvatarSrc('')).toBeUndefined();
    expect(normalizeAvatarSrc('   ')).toBeUndefined();
    expect(normalizeAvatarSrc('\n\t ')).toBeUndefined();
  });

  it('null / undefined 归一为 undefined', () => {
    expect(normalizeAvatarSrc(null)).toBeUndefined();
    expect(normalizeAvatarSrc(undefined)).toBeUndefined();
  });

  it('非字符串（防御后端类型漂移）归一为 undefined', () => {
    expect(normalizeAvatarSrc(123 as unknown as string)).toBeUndefined();
    expect(normalizeAvatarSrc({} as unknown as string)).toBeUndefined();
  });

  it('合法 URL 原样返回（并去掉首尾空白）', () => {
    expect(normalizeAvatarSrc('https://example.com/a.png')).toBe('https://example.com/a.png');
    expect(normalizeAvatarSrc('  https://example.com/a.png  ')).toBe('https://example.com/a.png');
  });

  it('两处 Avatar 调用点都走归一函数，不直接透传后端 avatar', () => {
    // hero 头像与头像弹窗若退回 src={profile?.avatar} / src={avatarValue}，
    // 空串又会漏回 DOM。这里做源码级锁定（渲染整个 Profile 需拉起 7 个 Tab
    // 与一批接口，代价与收益不成比例）。
    const dir = path.resolve(__dirname, '..');
    const read = (f: string) => fs.readFileSync(path.join(dir, f), 'utf8');
    expect(read('index.tsx')).toContain('src={normalizeAvatarSrc(profile?.avatar)}');
    expect(read('AvatarModal.tsx')).toContain('src={normalizeAvatarSrc(avatarValue)}');
    // 不允许出现未归一的直接透传
    expect(read('index.tsx')).not.toMatch(/src=\{profile\?\.avatar\}/);
    expect(read('AvatarModal.tsx')).not.toMatch(/src=\{avatarValue\}/);
  });
});
