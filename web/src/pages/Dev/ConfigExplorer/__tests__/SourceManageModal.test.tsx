/**
 * 数据源管理弹窗单测（覆盖率巡检：Dev/ConfigExplorer/SourceManageModal.tsx
 * 400 行 0% → 收口，ConfigExplorer 簇第二件；入口页另见 index.test.tsx，
 * 其中本组件为桩替身）。
 *
 * 锁定契约：
 * - open 门控加载：open && gameId && env 才拉列表（open=false 不拉）；
 *   列表矩阵：名称/类型/读写 Tag（可写橙/只读默认）/操作列 编辑/删除；
 *   标题 `管理数据源（{gameId} / {env}）`；load 失败静默文案；
 * - 新增主链：默认值（name 空/type git/config git 模板）、name required
 *   拦截（必填）、config JSON 校验失败翼（必须是合法 JSON）、type 切换 →
 *   formRef.setFieldValue 联动刷 config 模板（redis 模板）、提交载荷
 *   （id: undefined + gameId/env 上下文补齐）、「已保存」+ 重拉 + onChanged；
 * - 编辑主链：回填（name/type/config 脱敏值）、type Select disabled、
 *   脱敏提示文案、提交载荷（id 透传）、「已保存」；
 * - 保存失败：Error.message 透传 / 非 Error 兜底「保存失败」，表单保持打开；
 * - 删除主链：deleteConfigSource(id) →「已删除」+ 重拉 + onChanged；失败
 *   「删除失败」静默。
 *
 * mock 口径：services/api/configExplorer 三函数 jest.mock；@umijs/max 本地
 * mock（defaultMessage 即文案）；antd/pro-components 真实实现。
 *
 * 坑实证（antd6 沿用）：管理 Modal footer=null，嵌套 ModalForm 是页面唯一
 * 带 footer 的弹窗（主按钮锚 .ant-modal-footer .ant-btn-primary 全局唯一）；
 * 表单 type Select 是页面唯一 .ant-select（表格无下拉），mouseDown 根 + 点
 * 可见 option content；config TextArea 取页面唯一 textarea；ModalForm 标题
 * 与入口按钮同文本（添加数据源），锚 .ant-modal-title 逐个 textContent。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import SourceManageModal from '../SourceManageModal';
import type { ConfigSourceBinding } from '@/services/api/configExplorer';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/configExplorer', () => ({
  listConfigSources: jest.fn(),
  upsertConfigSource: jest.fn(),
  deleteConfigSource: jest.fn(),
  listConfigTree: jest.fn(),
  readConfigFile: jest.fn(),
  writeConfigFile: jest.fn(),
}));

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

import {
  listConfigSources,
  upsertConfigSource,
  deleteConfigSource,
} from '@/services/api/configExplorer';

const mList = listConfigSources as jest.MockedFunction<typeof listConfigSources>;
const mUpsert = upsertConfigSource as jest.MockedFunction<typeof upsertConfigSource>;
const mDelete = deleteConfigSource as jest.MockedFunction<typeof deleteConfigSource>;

const row1: ConfigSourceBinding = {
  id: 1,
  gameId: 'demo',
  env: 'prod',
  name: 'git-main',
  type: 'git',
  config: '{\n  "repoUrl": "******"\n}',
  writable: true,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
};
const row2: ConfigSourceBinding = {
  id: 2,
  gameId: 'demo',
  env: 'prod',
  name: 'redis-bus',
  type: 'redis',
  config: '{}',
  writable: false,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
};

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: [row1, row2] });
  mUpsert.mockResolvedValue(undefined);
  mDelete.mockResolvedValue(undefined);
});

function renderModal(open = true) {
  return render(
    <App>
      <SourceManageModal
        open={open}
        gameId="demo"
        env="prod"
        onClose={jest.fn()}
        onChanged={jest.fn()}
      />
    </App>,
  );
}

/** 等列表加载落定 */
async function waitLoad() {
  expect(await screen.findByText('git-main')).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledWith({ gameId: 'demo', env: 'prod' }));
}

function rowOf(text: string) {
  return screen.getByText(text).closest('tr') as HTMLElement;
}

/** 表单 footer 保存按钮（管理弹窗 footer=null，全局唯一主按钮） */
function submitForm() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/** 表单 type Select 选 option（页面唯一 .ant-select） */
async function pickTypeSelect(label: string) {
  await new Promise((r) => setTimeout(r, 60));
  const select = document.querySelector('.ant-select') as HTMLElement;
  fireEvent.mouseDown(select);
  const dropdown = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
      (d) => !d.className.includes('ant-select-dropdown-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  fireEvent.click(
    within(dropdown).getByText(label, { selector: '.ant-select-item-option-content' }),
  );
}

describe('数据源管理 列表', () => {
  it('open 门控加载：标题/表格矩阵/读写 Tag/操作列', async () => {
    renderModal();
    await waitLoad();

    expect(document.querySelector('.ant-modal-title')?.textContent).toBe(
      '管理数据源（demo / prod）',
    );
    expect(screen.getByText('添加数据源')).toBeInTheDocument();
    expect(screen.getByText('git')).toBeInTheDocument();
    expect(screen.getByText('redis')).toBeInTheDocument();
    expect(screen.getByText('可写').closest('.ant-tag')).toHaveClass('ant-tag-orange');
    expect(screen.getByText('只读').closest('.ant-tag')).not.toHaveClass('ant-tag-orange');
    expect(screen.getAllByText('编辑')).toHaveLength(2);
    expect(screen.getAllByText('删除')).toHaveLength(2);
  });

  it('open=false：不拉列表', async () => {
    renderModal(false);
    await new Promise((r) => setTimeout(r, 100));
    expect(mList).not.toHaveBeenCalled();
  });

  it('load 失败：静默文案不白屏', async () => {
    mList.mockRejectedValueOnce(new Error('src-down'));
    renderModal();
    expect(await screen.findByText('加载数据源失败')).toBeInTheDocument();
    expect(screen.getByText('添加数据源')).toBeInTheDocument();
  });
});

describe('数据源管理 新增', () => {
  it('默认值 + required/JSON 双拦截 + type 联动模板 + 载荷 → 已保存', async () => {
    renderModal();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '添加数据源' }));
    await waitFor(() => {
      const titles = Array.from(document.querySelectorAll('.ant-modal-title')).map(
        (t) => t.textContent,
      );
      expect(titles).toContain('添加数据源');
    });

    // 默认值：type git + git 模板 config
    expect(screen.getByDisplayValue(/repoUrl/)).toBeInTheDocument();

    // 空 name 提交 → required 拦截，不触达服务
    submitForm();
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mUpsert).not.toHaveBeenCalled();

    // 填 name + 清空 config → validator 空 value 短路 resolve（required 拦截）
    fireEvent.change(screen.getByPlaceholderText('如：数值表仓库 / skynet 配置总线'), {
      target: { value: '新数据源' },
    });
    const textarea = document.querySelector('.ant-modal textarea') as HTMLElement;
    fireEvent.change(textarea, { target: { value: '' } });
    submitForm();
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mUpsert).not.toHaveBeenCalled();

    // 非法 JSON → JSON 校验失败翼
    fireEvent.change(textarea, { target: { value: 'not-json' } });
    submitForm();
    expect(await screen.findByText('必须是合法 JSON')).toBeInTheDocument();
    expect(mUpsert).not.toHaveBeenCalled();

    // type 切 redis → config 联动刷 redis 模板
    await pickTypeSelect('Redis（skynet 配置总线）');
    await waitFor(() => expect(screen.getByDisplayValue(/addr/)).toBeInTheDocument());

    submitForm();
    await waitFor(() => expect(mUpsert).toHaveBeenCalledTimes(1));
    expect(mUpsert).toHaveBeenCalledWith(
      expect.objectContaining({
        gameId: 'demo',
        env: 'prod',
        name: '新数据源',
        type: 'redis',
      }),
    );
    const config = mUpsert.mock.calls[0][0].config;
    expect(config).toContain('addr');
    expect(JSON.parse(config)).toEqual(
      expect.objectContaining({ addr: '127.0.0.1:6379', prefix: 'cfg:' }),
    );
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('保存失败两翼：Error.message 透传 / 非 Error 兜底「保存失败」，表单保持', async () => {
    renderModal();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '添加数据源' }));
    await waitFor(() => {
      const titles = Array.from(document.querySelectorAll('.ant-modal-title')).map(
        (t) => t.textContent,
      );
      expect(titles).toContain('添加数据源');
    });
    fireEvent.change(screen.getByPlaceholderText('如：数值表仓库 / skynet 配置总线'), {
      target: { value: 'x' },
    });

    mUpsert.mockRejectedValueOnce(new Error('upsert-x'));
    submitForm();
    expect(await screen.findByText('upsert-x')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('如：数值表仓库 / skynet 配置总线')).toBeInTheDocument();

    mUpsert.mockRejectedValueOnce('plain' as never);
    submitForm();
    expect(await screen.findByText('保存失败')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('如：数值表仓库 / skynet 配置总线')).toBeInTheDocument();
  });
});

describe('数据源管理 编辑与删除', () => {
  it('编辑回填：脱敏 config + type 禁用 + 脱敏提示 + id 透传载荷', async () => {
    renderModal();
    await waitLoad();

    fireEvent.click(within(rowOf('git-main')).getByText('编辑'));
    await waitFor(() => {
      const titles = Array.from(document.querySelectorAll('.ant-modal-title')).map(
        (t) => t.textContent,
      );
      expect(titles).toContain('编辑：git-main');
    });

    expect(screen.getByDisplayValue('git-main')).toBeInTheDocument();
    expect(screen.getByDisplayValue(/repoUrl/)).toBeInTheDocument();
    // 编辑态 type Select 禁用
    expect(document.querySelector('.ant-select')).toHaveClass('ant-select-disabled');
    // 脱敏提示
    expect(
      screen.getByText('凭据字段已脱敏显示；无需更换凭据时保持 ****** 原样提交即可沿用旧值。'),
    ).toBeInTheDocument();

    submitForm();
    await waitFor(() => expect(mUpsert).toHaveBeenCalledTimes(1));
    expect(mUpsert).toHaveBeenCalledWith({
      id: 1,
      gameId: 'demo',
      env: 'prod',
      name: 'git-main',
      type: 'git',
      config: '{\n  "repoUrl": "******"\n}',
    });
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('删除主链：deleteConfigSource + 已删除 + 重拉；失败静默', async () => {
    renderModal();
    await waitLoad();

    fireEvent.click(within(rowOf('redis-bus')).getByText('删除'));
    await waitFor(() => expect(mDelete).toHaveBeenCalledWith(2));
    expect(await screen.findByText('已删除')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    mDelete.mockRejectedValueOnce(new Error('del-x'));
    fireEvent.click(within(rowOf('redis-bus')).getByText('删除'));
    expect(await screen.findByText('删除失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);
  });
});
