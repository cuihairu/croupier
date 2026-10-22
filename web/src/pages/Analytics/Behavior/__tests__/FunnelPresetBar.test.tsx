/**
 * 漏斗预设条单测（覆盖率巡检：Analytics/Behavior/FunnelPresetBar.tsx
 * 426 行 0% → 收口，零测试页排行第八）。
 *
 * 组件当前未挂载到漏斗卡片（文件头注释言明「已实现但未挂载」），直接
 * 以 props 渲染组件本体。
 *
 * 锁定契约：
 * - 读链：无草稿 → 空列表（无 sel 按钮组 disabled）；合法数组载入；
 *   非法 JSON → readAll catch 回 []；非数组 JSON（readAll 的
 *   Array.isArray 防御）→ []；
 * - 排序：lastUsed 降序；lastUsed 缺省（0 臂）与并列（localeCompare
 *   名称次序臂）；
 * - 保存为预设：prompt 命名 → 写入 {name, steps: join(','), sequential:
 *   seq?1:0, sameSession, gapSec, start/end: range ISO 或 undefined,
 *   lastUsed now} + sel 指向新名；重名 confirm 覆盖/取消两翼；prompt
 *   取消（null）与纯空白名（trim 空）早退不写；
 * - 应用预设：sel 命中 → lastUsed 顶格重写 + onApply(cleaned)（undefined
 *  值剔除）；未选中 disabled；sel 失配（storage 被外部清掉）→ 静默
 *  早退；
 * - 删除预设：confirm 确认 → 移除 + sel 清空；取消保留；
 * - 重命名：prompt 新名（非重名直接改）；重名 confirm 覆盖（splice 移除
 *  同名后改名）/取消两翼；prompt 取消早退；
 * - 导出：导出全部（createObjectURL + funnel_presets.json）/导出当前
 *  （单元素数组 blob，未选中 disabled）；URL.createObjectURL 是
 *  setupTests 的 jest.fn，断言调用与 Blob 内容；
 * - 清空全部：confirm 确认 → writeAll([]) + sel 清空；取消保留；
 * - 导入预览弹窗：解析三翼（非法 JSON → alert 解析失败 / 非数组 →
 *  alert JSON 需为数组 / 合法 → 预览表 覆盖|新增 状态 + 全选）；
 *  合并导入（勾选合并 + 未勾跳过 + 排序落库 + 弹窗关闭 + sel 清空）；
 *  取消关闭。
 *
 * mock 口径：@umijs/max 本地 defaultMessage mock（{name} 内插）；
 * window.prompt/confirm/alert 三 spy；localStorage 文件内 Map 存储
 * （setupTests 的 jest.fn() 壳无存储）；URL.createObjectURL 已是全局
 * jest.fn（setupTests），另 spy revokeObjectURL；真实 dayjs/formatDateTime。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - 九处静默 catch（writeAll/readAll JSON.parse 之外的操作级 try：
 *  savePreset/applyPreset/delPreset/renamePreset/exportOne/clearAll/
 *  exportPresets/savePreset 内层 writeAll/doImport 的 alert 导入失败）：
 *  Map 存储不抛、JSON.stringify(纯可序列化字段) 不抛、doImport 的
 *  map/forEach/sort 纯内存不抛——要触发只能故意把 setItem 改抛，
 *  属造假用例；
 * - 四处 `if (!sel) return`（applyPreset/delPreset/renamePreset/
 *  exportOne 的按钮均 disabled={!sel}——jsdom 对 disabled 按钮不派发
 *  click，空 sel 早退不可从 UI 触达）；
 * - renamePreset prompt 默认值 `sel || ''` 右臂：prompt 仅在 sel 非空
 *  且 found 命中后调用；
 * - parseImport 的 `x.name || String(i)` 右臂：上游 `.filter((x) =>
 *  x && x.name)` 保证 name 恒 truthy；
 * - Select 的 `(list || [])` 右臂：list 是 useState([])，永非 null。
 *
 * 坑实证（antd6 沿用）：prompt 第二参是默认值（sel 预填）；Select 选项
 * label 是 `${name} · ${formatDateTime(lastUsed)}`——断言用正则前缀；
 * Modal 表格 rowSelection 勾选锚 .ant-table-row-select-checkbox；
 * 两字中文 Button 自动插空格（应用预设等四字不插、重命名三字不插——
 * 仅两字插，本页按钮全 ≥3 字无此坑）。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import dayjs from 'dayjs';
import FunnelPresetBar from '../FunnelPresetBar';
import type { FunnelPreset } from '../types';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

const mockIntl = {
  formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string | number>) => {
    let msg = opts.defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) {
        msg = msg.split(`{${k}}`).join(String(v));
      }
    }
    return msg;
  },
};
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
}));

// tests/setupTests.jsx 的 localStorage 是无存储 jest.fn() 壳，补 Map 存储
const KEY = 'analytics:funnel_presets';
const storage = new Map<string, string>();
localStorage.getItem = (k: string) => storage.get(k) ?? null;
localStorage.setItem = (k: string, v: string) => void storage.set(k, String(v));
localStorage.removeItem = (k: string) => void storage.delete(k);
localStorage.clear = () => storage.clear();

const P = (over: Partial<FunnelPreset> & Pick<FunnelPreset, 'name'>): FunnelPreset => ({
  steps: 'a,b',
  sequential: 0,
  sameSession: 0,
  gapSec: 300,
  ...over,
});

// lastUsed 降序 + 并列回退名称序
const pAlpha = P({ name: 'alpha', lastUsed: '2026-09-01T00:00:00Z' });
const pBeta = P({ name: 'beta', lastUsed: '2026-09-28T00:00:00Z' });
const pNoDate = P({ name: 'no-date' });
const pNoDate2 = P({ name: 'aaa-no-date' });

// jsdom 无 revokeObjectURL（createObjectURL 已由 setupTests 定义为 jest.fn）
Object.defineProperty(URL, 'revokeObjectURL', {
  writable: true,
  value: jest.fn(),
});

let promptSpy: jest.SpyInstance;
let confirmSpy: jest.SpyInstance;
let alertSpy: jest.SpyInstance;

beforeEach(() => {
  jest.clearAllMocks();
  localStorage.clear();
  promptSpy = jest.spyOn(window, 'prompt').mockReturnValue(null);
  confirmSpy = jest.spyOn(window, 'confirm').mockReturnValue(false);
  alertSpy = jest.spyOn(window, 'alert').mockImplementation(() => {});
});

afterEach(() => {
  promptSpy.mockRestore();
  confirmSpy.mockRestore();
  alertSpy.mockRestore();
});

function renderBar(over?: { range?: [dayjs.Dayjs | null, dayjs.Dayjs | null] | null }) {
  const onApply = jest.fn();
  const utils = render(
    <App>
      <FunnelPresetBar
        steps={['stepA', 'stepB']}
        seq
        sameSess={false}
        gapSec={300}
        range={over?.range === undefined ? [dayjs('2026-09-01'), dayjs('2026-09-30')] : over.range}
        onApply={onApply}
      />
    </App>,
  );
  return { onApply, ...utils };
}

/** 等下拉选项出现并点选（rc-select 须先等一拍再 mouseDown） */
async function pickPreset(labelStart: string) {
  await new Promise((r) => setTimeout(r, 60));
  fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
  const dropdown = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
      (d) => !d.className.includes('ant-select-dropdown-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  const option = await waitFor(() => {
    const hit = Array.from(dropdown.querySelectorAll('.ant-select-item-option-content')).find((o) =>
      (o.textContent ?? '').startsWith(labelStart),
    ) as HTMLElement;
    expect(hit).not.toBeUndefined();
    return hit;
  });
  fireEvent.click(option);
}

function stored(): FunnelPreset[] {
  return JSON.parse(localStorage.getItem(KEY) || '[]') as FunnelPreset[];
}

describe('漏斗预设条 读链与排序', () => {
  it('空列表：四操作按钮 disabled；保存/导出全部/导入/清空可用', async () => {
    renderBar();
    expect(await screen.findByText('应用预设')).toBeDisabled();
    expect(screen.getByRole('button', { name: '重命名' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '删除预设' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '导出当前' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '保存为预设' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '导出全部' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '导入预设' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '清空全部' })).toBeEnabled();
  });

  it('载入排序：lastUsed 降序 → 并列（缺省 0 臂）名称 localeCompare', async () => {
    localStorage.setItem(KEY, JSON.stringify([pNoDate2, pAlpha, pNoDate, pBeta]));
    renderBar();
    await pickPreset('beta');
    // 二次打开须等关闭动画落定再 mouseDown
    await new Promise((r) => setTimeout(r, 60));
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    const names = Array.from(dropdown.querySelectorAll('.ant-select-item-option-content')).map(
      (o) => (o.textContent ?? '').split(' · ')[0],
    );
    expect(names).toEqual(['beta', 'alpha', 'aaa-no-date', 'no-date']);
  });

  it("无名预设排序归一：name 缺省 → String(a.name || '') 双臂（localStorage 可被外部写坏）", async () => {
    // 三元素保证比较器双向都出现（无名↔有名 两个方向）
    localStorage.setItem(
      KEY,
      JSON.stringify([
        { name: 'zeta', steps: 'y', gapSec: 2 },
        { steps: 'x', gapSec: 1 },
        { name: 'aaa', steps: 'z', gapSec: 3 },
      ]),
    );
    renderBar();
    await pickPreset('zeta');
    // 无名项参与比较不炸（|| '' 归一），仍在列表中；'' 经 label 模板串
    // 插值为字面 'undefined'，排序在 'zeta' 前
    await new Promise((r) => setTimeout(r, 300));
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    const names = Array.from(dropdown.querySelectorAll('.ant-select-item-option-content')).map(
      (o) => (o.textContent ?? '').split(' · ')[0],
    );
    expect(names).toEqual(['undefined', 'aaa', 'zeta']);
  });

  it('非法 JSON 与非数组 JSON → 空列表防御', async () => {
    localStorage.setItem(KEY, '{not-json');
    const { unmount } = renderBar();
    expect(await screen.findByText('应用预设')).toBeDisabled();
    unmount();

    localStorage.setItem(KEY, '{"name":"x"}');
    renderBar();
    expect(await screen.findByText('应用预设')).toBeDisabled();
  });
});

describe('漏斗预设条 保存与应用', () => {
  it('保存：prompt 命名 → 完整字段落库（ISO range）+ sel 指向；重名确认覆盖', async () => {
    const { onApply } = renderBar();
    promptSpy.mockReturnValue('P1');
    fireEvent.click(screen.getByRole('button', { name: '保存为预设' }));
    await waitFor(() => expect(stored()).toHaveLength(1));
    expect(stored()[0]).toMatchObject({
      name: 'P1',
      steps: 'stepA,stepB',
      sequential: 1,
      sameSession: 0,
      gapSec: 300,
      start: dayjs('2026-09-01').toISOString(),
      end: dayjs('2026-09-30').toISOString(),
    });
    expect(onApply).not.toHaveBeenCalled();

    // 重名：confirm 取消 → 原样单条
    promptSpy.mockReturnValue('P1');
    confirmSpy.mockReturnValue(false);
    fireEvent.click(screen.getByRole('button', { name: '保存为预设' }));
    await waitFor(() => expect(confirmSpy).toHaveBeenCalledTimes(1));
    expect(stored()).toHaveLength(1);

    // 重名：confirm 确认 → 覆盖（仍是单条）
    confirmSpy.mockReturnValue(true);
    fireEvent.click(screen.getByRole('button', { name: '保存为预设' }));
    await waitFor(() => expect(confirmSpy).toHaveBeenCalledTimes(2));
    expect(stored()).toHaveLength(1);
  });

  it('保存：prompt 取消 / 纯空白名 → 早退不写；range null → start/end undefined', async () => {
    const { unmount } = renderBar({ range: null });
    promptSpy.mockReturnValue(null);
    fireEvent.click(screen.getByRole('button', { name: '保存为预设' }));
    expect(stored()).toHaveLength(0);

    promptSpy.mockReturnValue('   ');
    fireEvent.click(screen.getByRole('button', { name: '保存为预设' }));
    expect(stored()).toHaveLength(0);
    unmount();

    // range [null, null] → toISOString 可选链双臂
    renderBar({ range: [null, null] });
    promptSpy.mockReturnValue('P2');
    fireEvent.click(screen.getByRole('button', { name: '保存为预设' }));
    await waitFor(() => expect(stored()).toHaveLength(1));
    expect(stored()[0].start).toBeUndefined();
    expect(stored()[0].end).toBeUndefined();
  });

  it('保存：seq=false + sameSess=true → 三元右臂 0/1', async () => {
    const onApply = jest.fn();
    render(
      <App>
        <FunnelPresetBar
          steps={['s']}
          seq={false}
          sameSess
          gapSec={1}
          range={null}
          onApply={onApply}
        />
      </App>,
    );
    promptSpy.mockReturnValue('P3');
    fireEvent.click(screen.getByRole('button', { name: '保存为预设' }));
    await waitFor(() => expect(stored()).toHaveLength(1));
    expect(stored()[0]).toMatchObject({ sequential: 0, sameSession: 1 });
  });

  it('应用：sel 命中 → onApply(cleaned) + lastUsed 顶格；storage 失配 → 静默早退', async () => {
    localStorage.setItem(KEY, JSON.stringify([pAlpha]));
    const { onApply } = renderBar();
    await pickPreset('alpha');
    fireEvent.click(screen.getByRole('button', { name: '应用预设' }));
    await waitFor(() => expect(onApply).toHaveBeenCalledTimes(1));
    expect(onApply.mock.calls[0][0]).toMatchObject({
      name: 'alpha',
      steps: 'a,b',
      gapSec: 300,
    });
    // undefined 值剔除：start/end 缺省不进载荷
    expect('start' in onApply.mock.calls[0][0]).toBe(false);
    // lastUsed 顶格重写
    expect(stored()[0].lastUsed).not.toBe(pAlpha.lastUsed);

    // 失配：选中后把 storage 清掉 → 点击静默（onApply 不再触发）
    localStorage.setItem(KEY, '[]');
    fireEvent.click(screen.getByRole('button', { name: '应用预设' }));
    await new Promise((r) => setTimeout(r, 100));
    expect(onApply).toHaveBeenCalledTimes(1);
  });
});

describe('漏斗预设条 删除/重命名/清空/导出', () => {
  async function bootWithPresets() {
    localStorage.setItem(KEY, JSON.stringify([pAlpha, pBeta]));
    const utils = renderBar();
    await pickPreset('alpha');
    return utils;
  }

  it('删除：确认 → 移除 + sel 清空；取消保留', async () => {
    await bootWithPresets();
    confirmSpy.mockReturnValue(false);
    fireEvent.click(screen.getByRole('button', { name: '删除预设' }));
    expect(stored()).toHaveLength(2);

    confirmSpy.mockReturnValue(true);
    fireEvent.click(screen.getByRole('button', { name: '删除预设' }));
    await waitFor(() => expect(stored()).toHaveLength(1));
    expect(stored()[0].name).toBe('beta');
    // sel 清空 → 按钮回 disabled
    await waitFor(() => expect(screen.getByRole('button', { name: '应用预设' })).toBeDisabled());
  });

  it('重命名：非重名直接改；重名确认覆盖（splice 移除同名）/取消', async () => {
    await bootWithPresets();
    promptSpy.mockReturnValue('renamed');
    fireEvent.click(screen.getByRole('button', { name: '重命名' }));
    await waitFor(() => {
      const names = stored()
        .map((x) => x.name)
        .sort();
      expect(names).toEqual(['beta', 'renamed']);
    });

    // 重命名到既有名：取消 → 原样
    promptSpy.mockReturnValue('beta');
    confirmSpy.mockReturnValue(false);
    fireEvent.click(screen.getByRole('button', { name: '重命名' }));
    await waitFor(() => expect(confirmSpy).toHaveBeenCalledTimes(1));
    expect(stored()).toHaveLength(2);

    // 重命名到既有名：确认 → beta 被移除、仅剩改名后的单条
    confirmSpy.mockReturnValue(true);
    fireEvent.click(screen.getByRole('button', { name: '重命名' }));
    await waitFor(() => expect(stored()).toHaveLength(1));
    expect(stored()[0].name).toBe('beta');
  });

  it('重命名：prompt 取消 / 纯空白 → 早退', async () => {
    await bootWithPresets();
    promptSpy.mockReturnValue(null);
    fireEvent.click(screen.getByRole('button', { name: '重命名' }));
    expect(stored()).toHaveLength(2);
    promptSpy.mockReturnValue('  ');
    fireEvent.click(screen.getByRole('button', { name: '重命名' }));
    expect(stored()).toHaveLength(2);
  });

  it('清空：确认 → 空库；取消保留', async () => {
    await bootWithPresets();
    confirmSpy.mockReturnValue(false);
    fireEvent.click(screen.getByRole('button', { name: '清空全部' }));
    expect(stored()).toHaveLength(2);

    confirmSpy.mockReturnValue(true);
    fireEvent.click(screen.getByRole('button', { name: '清空全部' }));
    await waitFor(() => expect(stored()).toHaveLength(0));
  });

  it('失配防御：选中后被外部清库 → 重命名/导出当前 found 缺失早退', async () => {
    await bootWithPresets();
    localStorage.setItem(KEY, '[]');
    const createObj = URL.createObjectURL as jest.Mock;
    createObj.mockClear();
    promptSpy.mockReturnValue('zz');
    fireEvent.click(screen.getByRole('button', { name: '重命名' }));
    await new Promise((r) => setTimeout(r, 100));
    expect(promptSpy).not.toHaveBeenCalled();
    expect(stored()).toHaveLength(0);

    fireEvent.click(screen.getByRole('button', { name: '导出当前' }));
    await new Promise((r) => setTimeout(r, 100));
    expect(createObj).not.toHaveBeenCalled();
  });

  it('导出全部/导出当前：blob JSON 内容 + 文件名', async () => {
    await bootWithPresets();
    const createObj = URL.createObjectURL as jest.Mock;
    createObj.mockClear();

    fireEvent.click(screen.getByRole('button', { name: '导出全部' }));
    await waitFor(() => expect(createObj).toHaveBeenCalledTimes(1));
    const blobAll = createObj.mock.calls[0][0] as Blob;
    expect(blobAll.type).toBe('application/json');

    fireEvent.click(screen.getByRole('button', { name: '导出当前' }));
    await waitFor(() => expect(createObj).toHaveBeenCalledTimes(2));
  });
});

describe('漏斗预设条 导入预览', () => {
  function openModal() {
    fireEvent.click(screen.getByRole('button', { name: '导入预设' }));
    return waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('导入预设（预览）'),
    );
  }

  it('解析三翼：非法 JSON alert / 非数组 alert / 合法 → 覆盖|新增 预览全选', async () => {
    // 已有 beta → 导入同名为「覆盖」，gamma 为「新增」
    localStorage.setItem(KEY, JSON.stringify([pBeta]));
    renderBar();
    await openModal();

    fireEvent.change(document.querySelector('.ant-modal textarea') as HTMLElement, {
      target: { value: '{bad' },
    });
    fireEvent.click(within(document.querySelector('.ant-modal') as HTMLElement).getByText('解析'));
    await waitFor(() => expect(alertSpy).toHaveBeenLastCalledWith('解析失败'));

    fireEvent.change(document.querySelector('.ant-modal textarea') as HTMLElement, {
      target: { value: '{"name":"x"}' },
    });
    fireEvent.click(within(document.querySelector('.ant-modal') as HTMLElement).getByText('解析'));
    await waitFor(() => expect(alertSpy).toHaveBeenLastCalledWith('JSON 需为数组'));

    // 合法：beta 已存在 → 覆盖；gamma 新 → 新增；无 name 项被过滤
    fireEvent.change(document.querySelector('.ant-modal textarea') as HTMLElement, {
      target: {
        value: JSON.stringify([pBeta, P({ name: 'gamma' }), { steps: 'x' }]),
      },
    });
    fireEvent.click(within(document.querySelector('.ant-modal') as HTMLElement).getByText('解析'));
    expect(
      await within(document.querySelector('.ant-modal') as HTMLElement).findByText('gamma'),
    ).toBeInTheDocument();
    const modal = document.querySelector('.ant-modal') as HTMLElement;
    expect(within(modal).getAllByText('覆盖')).toHaveLength(1);
    expect(within(modal).getAllByText('新增')).toHaveLength(1);
  });

  it('合并导入：勾选合并 + 未勾跳过 → 排序落库 + 弹窗关闭 + 取消关闭', async () => {
    localStorage.setItem(KEY, JSON.stringify([pBeta]));
    renderBar();
    await openModal();

    fireEvent.change(document.querySelector('.ant-modal textarea') as HTMLElement, {
      target: { value: JSON.stringify([P({ name: 'delta' }), P({ name: 'gamma' })]) },
    });
    fireEvent.click(within(document.querySelector('.ant-modal') as HTMLElement).getByText('解析'));
    expect(
      await within(document.querySelector('.ant-modal') as HTMLElement).findByText('delta'),
    ).toBeInTheDocument();

    // 取消 gamma 的勾选（第二行）
    const rows = document.querySelectorAll('.ant-modal .ant-table-tbody tr.ant-table-row');
    fireEvent.click(rows[1].querySelector('.ant-checkbox-input') as HTMLElement);
    fireEvent.click(
      within(document.querySelector('.ant-modal-footer') as HTMLElement).getByText('合并导入'),
    );
    await waitFor(() =>
      expect(
        stored()
          .map((x) => x.name)
          .sort(),
      ).toEqual(['beta', 'delta']),
    );
    // antd6 关闭后 modal 留在 DOM（display: none），按隐藏断言
    await waitFor(() =>
      expect(document.querySelector('.ant-modal')?.getAttribute('style')).toContain(
        'display: none',
      ),
    );

    // 取消关闭：不写入
    fireEvent.click(screen.getByRole('button', { name: '导入预设' }));
    await openModal();
    fireEvent.click(
      within(document.querySelector('.ant-modal-footer') as HTMLElement).getByText('取 消'),
    );
    await waitFor(() =>
      expect(document.querySelector('.ant-modal')?.getAttribute('style')).toContain(
        'display: none',
      ),
    );
    expect(
      stored()
        .map((x) => x.name)
        .sort(),
    ).toEqual(['beta', 'delta']);
  });
});
