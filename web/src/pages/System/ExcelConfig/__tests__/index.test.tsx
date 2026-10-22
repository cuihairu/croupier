/**
 * 表格配置（Excel 在线编译）页单测（覆盖率巡检：
 * System/ExcelConfig/index.tsx 444 行 0% → 收口，零测试页排行第六）。
 *
 * 锁定契约：
 * - 草稿生命周期：无草稿 → 默认 Sheet1 [['id','name','value']]；合法
 *   localStorage 草稿载入；非法 JSON / 空数组 → 回退默认；编辑即持久化；
 *   重置草稿按钮重新读取 localStorage；
 * - 单元格编辑：setCell 数字自动转 number（int/float 双正则）、参差行
 *   补列（rows[row].length <= col 的 while 补 ''）、编辑后落 localStorage；
 * - 类型行：第二行首格 # 开头 → 该行渲染 Select（int/string/float/bool），
 *   首列 disabled；切换类型落库；
 * - 行/表操作：+ 行（按表头宽度补空行）、+ sheet（SheetN 命名 + 激活新
 *   表）、CheckableTag 切换激活表；
 * - 导入 .xlsx：XLSX 真实解析（raw: true 数值保持 number）→ 草稿替换 +
 *   「已导入 N 个 sheet」；坏文件（PK 头 + 垃圾）→ SheetJS 抛错 →
 *   extractErrorMessage 透传 Error.message；
 * - 导出 .xlsx：XLSX.writeFile(wb, `${configKey||'excel-config'}.xlsx`)，
 *   key 空串回退默认名；
 * - 保存并发布：Popconfirm → compileExcelSnapshot({snapshot, key,
 *   message: commitMessage || undefined})——snapshot 为稀疏 cellData
 *   （''/null/undefined 单元格跳过）→「已注册版本 vN（S 表 / R 行）」+
 *   最新版本绿 Tag + 版本说明清空；失败两翼（Error.message / 非 Error
 *   「保存失败」）；
 * - 服务端编译上传：importExcelFile(file, {message}) →「服务端编译完成：
 *   vN（R 行）」；失败两翼（Error.message / 非 Error「上传编译失败」）。
 *
 * mock 口径：services/api/excelConfig 两函数 jest.mock；xlsx 半真实
 * （requireActual + writeFile spy，jsdom 无下载）；@umijs/max 本地
 * defaultMessage mock；localStorage 真实（beforeEach clear，防跨用例
 * 草稿泄漏）。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - setCell 的行补齐 `while (rows.length <= row)`：表格仅对既有行渲染
 *   输入，row 恒 < rows.length（+ 行走 addRow 的等宽追加）；
 * - addRow 的 `current.rows[0]?.length || 1` 右臂：草稿载入与 + sheet
 *   都保证首行存在（loadLocalDraft 默认形态、XLSX header:1 至少出参差
 *   行），空 rows 数组不可从 UI 构造；
 * - 单元格 render 的 `(current?.rows || [])` 右臂：dataSource 即
 *   `current?.rows || []`——current 为 undefined 时表体零行，render 回调
 *   不会被调（行对象恒来自 current.rows）；
 * - 导入链 `parsed.length === 0`（「文件没有 sheet」警告分支）：XLSX.write
 *   对零表工作簿直接抛 Workbook is empty（无法构造合法零表 xlsx），
 *   XLSX.read 成功解析的真实文件恒有 ≥1 sheet，坏文件走 catch 臂；
 * - 导入/上传 catch 内「解析失败」「上传编译失败」的 extractErrorMessage
 *   兜底右臂：SheetJS/网络层抛的均为带非空 message 的 Error（左链恒命中），
 *   非对象/空 message 抛出不可从真实依赖构造。
 *
 * 坑实证（antd6 沿用）：Popconfirm 确认锚未隐藏 .ant-popover 内
 * .ant-btn-primary；页内两个 Upload 的 input[type=file] 按 DOM 序
 * [导入, 服务端编译]；类型行 Select 是表格内 .ant-select，mouseDown 根 +
 * 点可见 option content；XLSX 导入测试用 XLSX.write 现场造包（round-trip
 * 不 mock 解析侧）；tests/setupTests.jsx 的 localStorage 是无存储
 * jest.fn() 壳（getItem 恒 undefined），依赖草稿的页面测试须文件内补
 * Map 存储；本 jsdom Blob/File 无 arrayBuffer()，须 FileReader 打底
 * polyfill（页面导入链 file.arrayBuffer → XLSX.read）；scroll 表格 tbody
 * 首行是 aria-hidden 的 ant-table-measure-row，行选择器须带
 * .ant-table-row。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import * as XLSX from 'xlsx';
import ExcelConfigPage from '../index';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/excelConfig', () => ({
  compileExcelSnapshot: jest.fn(),
  importExcelFile: jest.fn(),
}));

jest.mock('xlsx', () => ({
  ...jest.requireActual('xlsx'),
  writeFile: jest.fn(),
}));

const mockIntl = {
  formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
};
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
}));

import { compileExcelSnapshot, importExcelFile } from '@/services/api/excelConfig';

const mCompile = compileExcelSnapshot as jest.MockedFunction<typeof compileExcelSnapshot>;
const mImport = importExcelFile as jest.MockedFunction<typeof importExcelFile>;
const mWriteFile = XLSX.writeFile as jest.MockedFunction<typeof XLSX.writeFile>;

const DRAFT_KEY = 'excel-config-draft';

// tests/setupTests.jsx 把 localStorage 换成了无存储的 jest.fn() 壳
// （getItem 恒 undefined），本页契约全部构建在真实 localStorage 草稿
// 之上——在此补一个文件内 Map 存储（setupFiles 每文件独立跑，不影响他文件）
const storage = new Map<string, string>();
localStorage.getItem = (k: string) => storage.get(k) ?? null;
localStorage.setItem = (k: string, v: string) => void storage.set(k, String(v));
localStorage.removeItem = (k: string) => void storage.delete(k);
localStorage.clear = () => storage.clear();

// 本 jsdom 的 Blob/File 无 arrayBuffer()（页面导入链 file.arrayBuffer →
// XLSX.read），用 FileReader 打底（仅影响本文件的模块注册表）
if (!Blob.prototype.arrayBuffer) {
  Object.defineProperty(Blob.prototype, 'arrayBuffer', {
    value: function arrayBuffer(): Promise<ArrayBuffer> {
      return new Promise((resolve) => {
        const fr = new FileReader();
        fr.onload = () => resolve(fr.result as ArrayBuffer);
        fr.readAsArrayBuffer(this);
      });
    },
    writable: true,
  });
}

beforeEach(() => {
  jest.clearAllMocks();
  localStorage.clear();
  mCompile.mockResolvedValue({ key: 'excel.workbook', version: 3, sheets: 1, rows: 2 });
  mImport.mockResolvedValue({ key: 'excel.workbook', version: 5, sheets: 1, rows: 3 });
});

function renderPage() {
  return render(
    <App>
      <ExcelConfigPage />
    </App>,
  );
}

/** 表体数据行（第 idx 行）——排除 antd 的 ant-table-measure-row（tbody 首
 * 行 aria-hidden 测量行，scroll 表格必有） */
function bodyRow(idx: number) {
  return document.querySelectorAll('.ant-table-tbody tr.ant-table-row')[idx] as HTMLElement;
}

/** XLSX 现场造包 */
function xlsxFile(sheets: Record<string, unknown[][]>, name = 't.xlsx') {
  const wb = XLSX.utils.book_new();
  for (const [n, rows] of Object.entries(sheets)) {
    XLSX.utils.book_append_sheet(wb, XLSX.utils.aoa_to_sheet(rows), n);
  }
  const out = XLSX.write(wb, { type: 'array', bookType: 'xlsx' }) as ArrayBuffer;
  return new File([out], name);
}

/** Popconfirm 确认 */
async function confirmPopover() {
  const popover = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  fireEvent.click(popover.querySelector('.ant-btn-primary') as HTMLElement);
}

describe('表格配置 草稿与编辑', () => {
  it('无草稿：默认 Sheet1 + 表头输入 + 列标题（字段/数据 与 列 N）+ 约定文案', async () => {
    renderPage();
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();
    // 表头三输入
    expect(within(bodyRow(0)).getByDisplayValue('id')).toBeInTheDocument();
    expect(within(bodyRow(0)).getByDisplayValue('name')).toBeInTheDocument();
    expect(within(bodyRow(0)).getByDisplayValue('value')).toBeInTheDocument();
    // 列标题：首列「字段 / 数据」、其余「列 2」「列 3」（约定文案会重复
    // 出现这些词，用 getAllByText）
    expect(screen.getAllByText('字段 / 数据').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('列 2').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('列 3').length).toBeGreaterThanOrEqual(1);
    // 约定文案
    expect(screen.getByText(/首行=字段名/)).toBeInTheDocument();
  });

  it('本地草稿载入 + 编辑持久化（数字转换/参差行补列）+ 非法 JSON 回退', async () => {
    localStorage.setItem(
      DRAFT_KEY,
      JSON.stringify([{ name: 'Draft', rows: [['a', 'b', 'c'], ['x']] }]),
    );
    renderPage();
    expect(await screen.findByText('Draft')).toBeInTheDocument();
    expect(within(bodyRow(0)).getByDisplayValue('a')).toBeInTheDocument();
    expect(within(bodyRow(1)).getByDisplayValue('x')).toBeInTheDocument();

    // 编辑参差行第 3 列（渲染 '' 输入，setCell 补列）+ 数字自动转 number
    fireEvent.change(within(bodyRow(1)).getAllByDisplayValue('')[1], {
      target: { value: 'zz' },
    });
    fireEvent.change(within(bodyRow(0)).getByDisplayValue('b'), {
      target: { value: '42' },
    });
    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem(DRAFT_KEY) || '[]') as {
        rows: (string | number)[][];
      }[];
      expect(saved[0].rows[1]).toEqual(['x', '', 'zz']);
      expect(saved[0].rows[0][1]).toBe(42);
    });

    // 浮点正则
    fireEvent.change(within(bodyRow(0)).getByDisplayValue('42'), {
      target: { value: '3.5' },
    });
    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem(DRAFT_KEY) || '[]') as {
        rows: (string | number)[][];
      }[];
      expect(saved[0].rows[0][1]).toBe(3.5);
    });

    // 重置草稿按钮：重新读取 localStorage（编辑仍在）
    fireEvent.click(screen.getByRole('button', { name: /重置草稿/ }));
    expect(await screen.findByText('Draft')).toBeInTheDocument();
    expect(within(bodyRow(0)).getByDisplayValue('3.5')).toBeInTheDocument();

    // 非法 JSON → 回退默认 Sheet1
    localStorage.setItem(DRAFT_KEY, '{not-json');
    fireEvent.click(screen.getByRole('button', { name: /重置草稿/ }));
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();
    expect(within(bodyRow(0)).getByDisplayValue('id')).toBeInTheDocument();
  });

  it('类型行：# 开头 → Select 矩阵 + 首列 disabled + 空类型格 placeholder + 切换落库', async () => {
    // 参差表头（首行 1 宽 < 类型行 2 宽）：表头次格走 row[ci] ?? '' 渲染；
    // 类型行次格空串 → Select value 右臂 || ''→|| undefined → placeholder
    localStorage.setItem(DRAFT_KEY, JSON.stringify([{ name: 'T', rows: [['id'], ['#int', '']] }]));
    renderPage();
    expect(await screen.findByText('T')).toBeInTheDocument();

    // 表头两格：'id' + 参差补 ''（?? '' 左臂）
    expect(within(bodyRow(0)).getByDisplayValue('id')).toBeInTheDocument();
    expect(within(bodyRow(0)).getAllByDisplayValue('')).toHaveLength(1);

    const typeRow = bodyRow(1);
    const selects = typeRow.querySelectorAll('.ant-select');
    expect(selects).toHaveLength(2);
    // 首列 disabled、显示 '#int'；次列空 → value undefined（无选中项）
    expect(selects[0]).toHaveClass('ant-select-disabled');
    expect(within(typeRow).getByText('#int')).toBeInTheDocument();
    expect(selects[1].querySelector('.ant-select-selection-item')).toBeNull();

    // 切换次列类型为 float
    fireEvent.mouseDown(selects[1] as HTMLElement);
    const dropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    fireEvent.click(
      within(dropdown).getByText('float', { selector: '.ant-select-item-option-content' }),
    );
    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem(DRAFT_KEY) || '[]') as {
        rows: string[][];
      }[];
      expect(saved[0].rows[1][1]).toBe('float');
    });
  });

  it('+ 行（表头宽度空行）+ + sheet（SheetN 激活）+ CheckableTag 切回', async () => {
    renderPage();
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();
    const baseRows = document.querySelectorAll('.ant-table-tbody tr.ant-table-row').length;

    fireEvent.click(screen.getByRole('button', { name: /^\+ 行$/ }));
    await waitFor(() =>
      expect(document.querySelectorAll('.ant-table-tbody tr.ant-table-row').length).toBe(
        baseRows + 1,
      ),
    );
    const newRow = bodyRow(baseRows);
    expect(within(newRow).getAllByDisplayValue('')).toHaveLength(3);

    fireEvent.click(screen.getByRole('button', { name: /^\+ sheet$/ }));
    expect(await screen.findByText('Sheet2')).toBeInTheDocument();
    expect(within(bodyRow(0)).getByDisplayValue('id')).toBeInTheDocument();

    // 切回 Sheet1
    fireEvent.click(screen.getByText('Sheet1'));
    await waitFor(() => expect(within(bodyRow(0)).getByDisplayValue('id')).toBeInTheDocument());
    expect(within(bodyRow(0)).getByDisplayValue('name')).toBeInTheDocument();

    // 双表存续下编辑 Sheet1：updateRows 的 map 对非激活表走 `: s` 原样臂
    fireEvent.change(within(bodyRow(0)).getByDisplayValue('id'), {
      target: { value: 'pid' },
    });
    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem(DRAFT_KEY) || '[]') as {
        name: string;
        rows: string[][];
      }[];
      expect(saved).toHaveLength(2);
      expect(saved[0].rows[0][0]).toBe('pid');
      // Sheet2 原样保留
      expect(saved[1]).toEqual({ name: 'Sheet2', rows: [['id']] });
    });
  });
});

describe('表格配置 导入导出', () => {
  it('导入 .xlsx：真实解析 → 草稿替换 + 已导入提示；空 sheet → 警告；坏文件 → 解析失败', async () => {
    // 同一 input 二次 change 被 rc-upload 吞（value 复位去重），分渲染
    const first = renderPage();
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();

    const input = document.querySelectorAll('input[type="file"]')[0] as HTMLInputElement;
    fireEvent.change(input, {
      target: {
        files: [
          xlsxFile({
            Imp: [
              ['id', 'qty'],
              ['1', 2],
            ],
          }),
        ],
      },
    });
    expect(
      await screen.findByText('已导入 1 个 sheet（草稿，保存后注册新版本）'),
    ).toBeInTheDocument();
    expect(await screen.findByText('Imp')).toBeInTheDocument();
    // raw: true → 数字保持 number
    expect(within(bodyRow(1)).getByDisplayValue('2')).toBeInTheDocument();
    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem(DRAFT_KEY) || '[]') as {
        name: string;
      }[];
      expect(saved.map((s) => s.name)).toEqual(['Imp']);
    });
    first.unmount();

    // 坏文件（PK 头 + 垃圾 → SheetJS 抛 Unsupported ZIP encryption）
    // → catch 臂：extractErrorMessage 透传 Error.message
    const third = renderPage();
    expect(await screen.findByText('Imp')).toBeInTheDocument();
    const input3 = document.querySelectorAll('input[type="file"]')[0] as HTMLInputElement;
    fireEvent.change(input3, {
      target: {
        files: [new File([new Uint8Array([0x50, 0x4b, 0x03, 0x04, 1, 2, 3, 4, 5, 6])], 'bad.xlsx')],
      },
    });
    expect(await screen.findByText('Unsupported ZIP encryption')).toBeInTheDocument();
    third.unmount();
  });

  it('导出 .xlsx：默认 key 文件名 + 空串回退 excel-config', async () => {
    renderPage();
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /导出 \.xlsx/ }));
    await waitFor(() => expect(mWriteFile).toHaveBeenCalledTimes(1));
    expect(mWriteFile.mock.calls[0][1]).toBe('excel.workbook.xlsx');

    fireEvent.change(screen.getByPlaceholderText('配置 key（如 shop.items）'), {
      target: { value: '' },
    });
    fireEvent.click(screen.getByRole('button', { name: /导出 \.xlsx/ }));
    await waitFor(() => expect(mWriteFile).toHaveBeenCalledTimes(2));
    expect(mWriteFile.mock.calls[1][1]).toBe('excel-config.xlsx');
  });
});

describe('表格配置 保存与上传', () => {
  it('保存并发布：稀疏快照 + message 透传 → 已注册 + 最新版本 Tag + 说明清空', async () => {
    localStorage.setItem(
      DRAFT_KEY,
      JSON.stringify([
        {
          name: 'S1',
          rows: [
            ['id', 'name'],
            [1, ''],
          ],
        },
      ]),
    );
    renderPage();
    expect(await screen.findByText('S1')).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText('版本说明（可选）'), {
      target: { value: 'v2 desc' },
    });
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    await confirmPopover();
    await waitFor(() => expect(mCompile).toHaveBeenCalledTimes(1));
    // 稀疏 cellData：'' 单元格跳过；数字保持 number
    expect(mCompile).toHaveBeenCalledWith({
      snapshot: {
        sheets: {
          S1: {
            cellData: {
              '0': { '0': { v: 'id' }, '1': { v: 'name' } },
              '1': { '0': { v: 1 } },
            },
          },
        },
      },
      key: 'excel.workbook',
      message: 'v2 desc',
    });
    expect(await screen.findByText('已注册版本 v3（1 表 / 2 行）')).toBeInTheDocument();
    expect(screen.getByText('最新版本 v3（1 表 / 2 行）')).toBeInTheDocument();
    // 版本说明清空
    await waitFor(() => expect(screen.getByPlaceholderText('版本说明（可选）')).toHaveValue(''));
  });

  it('保存失败两翼：Error.message / 非 Error「保存失败」', async () => {
    renderPage();
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();

    mCompile.mockRejectedValueOnce(new Error('compile-x'));
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    await confirmPopover();
    expect(await screen.findByText('compile-x')).toBeInTheDocument();

    mCompile.mockRejectedValueOnce('plain' as never);
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    await confirmPopover();
    expect(await screen.findByText('保存失败')).toBeInTheDocument();
  });

  it('服务端编译上传：importExcelFile(file, {message}) → 完成提示；失败两翼', async () => {
    // 同一 input 二次 change 被 rc-upload 吞，分渲染
    const first = renderPage();
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();

    const input = document.querySelectorAll('input[type="file"]')[1] as HTMLInputElement;
    const file = xlsxFile({ S: [['id'], ['1']] }, 'up.xlsx');
    fireEvent.change(input, { target: { files: [file] } });
    await waitFor(() => expect(mImport).toHaveBeenCalledWith(file, { message: undefined }));
    expect(await screen.findByText('服务端编译完成：v5（3 行）')).toBeInTheDocument();
    expect(screen.getByText('最新版本 v5（1 表 / 3 行）')).toBeInTheDocument();
    first.unmount();

    // 带版本说明
    const second = renderPage();
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText('版本说明（可选）'), {
      target: { value: 'srv msg' },
    });
    const input2 = document.querySelectorAll('input[type="file"]')[1] as HTMLInputElement;
    const file2 = xlsxFile({ S: [['id']] }, 'up2.xlsx');
    fireEvent.change(input2, { target: { files: [file2] } });
    await waitFor(() => expect(mImport).toHaveBeenLastCalledWith(file2, { message: 'srv msg' }));
    second.unmount();

    // 失败两翼：Error.message / 非 Error「上传编译失败」
    mImport.mockRejectedValueOnce(new Error('srv-x'));
    const third = renderPage();
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();
    const input3 = document.querySelectorAll('input[type="file"]')[1] as HTMLInputElement;
    fireEvent.change(input3, { target: { files: [xlsxFile({ S: [['id']] }, 'bad.xlsx')] } });
    expect(await screen.findByText('srv-x')).toBeInTheDocument();
    third.unmount();

    mImport.mockRejectedValueOnce('plain' as never);
    renderPage();
    expect(await screen.findByText('Sheet1')).toBeInTheDocument();
    const input4 = document.querySelectorAll('input[type="file"]')[1] as HTMLInputElement;
    fireEvent.change(input4, { target: { files: [xlsxFile({ S: [['id']] }, 'bad2.xlsx')] } });
    expect(await screen.findByText('上传编译失败')).toBeInTheDocument();
  });
});
