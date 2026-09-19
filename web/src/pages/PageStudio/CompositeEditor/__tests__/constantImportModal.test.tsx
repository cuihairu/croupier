/**
 * ConstantImportModal（导入常量向导）：
 * JSON 导入（对象映射/数组形态/空结果提示/解析失败 Alert）、
 * Excel 导入（XLSX 真实字节流，长表聚合）、
 * 保存（空字段提示、逐常量 POST component-templates、成功回调与重置、失败 Alert）、
 * 取消（重置 + onCancel）。
 *
 * 补充覆盖（v8 branch/function 缺口）：
 * - Excel 解析失败 catch（PNG 魔数字节确定性触发 XLSX.read 抛错）
 * - JSON 解析抛非 Error 值 → 「JSON 解析失败」兜底（jsonToFields mock 单次抛字符串）
 * - 空名常量：save 时 f.title || f.key 双兜底分支（name 与 tree 标题回退）
 * - 保存 reject 非 Error 值 → 「保存失败」兜底
 * - Modal 右上角 X（onCancel：reset + onCancel 回调）
 * - ConstantFieldsEditor onChange → schemaToFields 回写 fields（计数联动）
 */
import React from 'react';
import { fireEvent, render, screen, waitFor, configure } from '@testing-library/react';
import * as XLSX from 'xlsx';
import { request } from '@umijs/max';
import ConstantImportModal from '../ConstantImportModal';
import { jsonToFields } from '../constants';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

// @umijs/max 用 setupTests 的全局 mock（request 即 jest.fn）
const requestMock = request as unknown as jest.Mock;

// jsonToFields 包一层 jest.fn（默认透传真实实现），供「抛非 Error 异常」
// 用例 mockImplementationOnce 单次替换，其余用例行为不变
jest.mock('../constants', () => {
  const actual = jest.requireActual('../constants');
  return { ...actual, jsonToFields: jest.fn(actual.jsonToFields) };
});

jest.mock('../ConstantFieldsEditor', () => ({
  __esModule: true,
  default: (props: { value?: string; onChange?: (v: string) => void }) => {
    // 模拟编辑器提交：点击按钮回写一份单字段 schema（驱动 onChange → schemaToFields 链路）
    const singleFieldSchema = JSON.stringify({
      type: 'object',
      properties: { 阵营: { type: 'string', title: '阵营', enum: ['联盟', '部落'] } },
    });
    return (
      <div data-testid="constant-fields-editor">
        <button
          type="button"
          data-testid="cfe-change"
          onClick={() => props.onChange?.(singleFieldSchema)}
        >
          模拟编辑器回写
        </button>
      </div>
    );
  },
}));

function renderModal(onSaved = jest.fn()) {
  const onCancel = jest.fn();
  const utils = render(<ConstantImportModal open onCancel={onCancel} onSaved={onSaved} />);
  return { onSaved, onCancel, ...utils };
}

/** 通过 antd Upload 渲染的 input[type=file] 触发 beforeUpload */
function uploadFile(file: File) {
  const input = document.querySelector('input[type=file]') as HTMLInputElement;
  fireEvent.change(input, { target: { files: [file] } });
}

function jsonFile(content: string): File {
  return new File([content], 'consts.json', { type: 'application/json' });
}

/** 用 XLSX 生成真实 .xlsx 字节流（sheet 名固定 Sheet1） */
function xlsxFile(rows: string[][]): File {
  const ws = XLSX.utils.aoa_to_sheet(rows);
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, 'Sheet1');
  const buf = XLSX.write(wb, { type: 'array', bookType: 'xlsx' }) as ArrayBuffer;
  return new File([buf], 'consts.xlsx', {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  });
}

beforeEach(() => {
  jest.clearAllMocks();
  requestMock.mockReset();
});

describe('JSON 导入', () => {
  it('对象映射形态导入成功并进入预览', async () => {
    renderModal();
    uploadFile(jsonFile('{"阵营":["联盟","部落"]}'));

    await waitFor(() => expect(screen.getByText(/常量预览（1 个/)).toBeInTheDocument());
    expect(requestMock).not.toHaveBeenCalled();
  });

  it('数组形态导入成功', async () => {
    renderModal();
    uploadFile(jsonFile('[{"name":"稀有度","options":["传说","史诗"]}]'));

    await waitFor(() => expect(screen.getByText(/常量预览（1 个/)).toBeInTheDocument());
  });

  it('空结果（无可识别常量）提示 JSON 格式要求', async () => {
    renderModal();
    uploadFile(jsonFile('{"不相关": 42}'));

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('JSON 需为'));
    expect(screen.queryByText(/常量预览/)).not.toBeInTheDocument();
  });

  it('非法 JSON 弹解析错误 Alert', async () => {
    renderModal();
    uploadFile(jsonFile('{oops'));

    await waitFor(() => screen.getByRole('alert'));
    expect(screen.getByRole('alert')).toHaveTextContent(/Unexpected|JSON/i);
  });
});

describe('Excel 导入', () => {
  it('长表：同名称多行聚合为一个常量', async () => {
    renderModal();
    // rowsToFields 不识别表头行，直接放数据行（名称|值|标签）
    uploadFile(
      xlsxFile([
        ['阵营', 'alliance', '联盟'],
        ['阵营', 'horde', '部落'],
      ]),
    );

    await waitFor(() => expect(screen.getByText(/常量预览（1 个/)).toBeInTheDocument());
  });

  it('宽表：逐行一个常量', async () => {
    renderModal();
    fireEvent.click(screen.getByText('宽表：名称|选项…'));
    uploadFile(
      xlsxFile([
        ['服务器', 'S1', 'S2'],
        ['渠道', 'ios'],
      ]),
    );

    await waitFor(() => expect(screen.getByText(/常量预览（2 个/)).toBeInTheDocument());
  });
});

describe('保存', () => {
  it('未导入常量时提示先导入', async () => {
    renderModal();
    fireEvent.click(screen.getByRole('button', { name: /全部保存（0 个组件）/ }));

    await waitFor(() =>
      expect(screen.getByText('请先导入常量（Excel/JSON）或添加字段')).toBeInTheDocument(),
    );
    expect(requestMock).not.toHaveBeenCalled();
  });

  it('每个常量独立 POST 保存，成功后回调并重置', async () => {
    const onSaved = jest.fn();
    renderModal(onSaved);
    uploadFile(jsonFile('{"阵营":["联盟","部落"],"稀有度":["传说"]}'));

    const saveButton = await screen.findByRole('button', {
      name: /全部保存（2 个组件）/,
    });
    expect(saveButton).toBeInTheDocument();
    fireEvent.click(saveButton);

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('2 个常量组件'));
    expect(requestMock).toHaveBeenCalledTimes(2);
    const firstBody = requestMock.mock.calls[0][1].data as {
      key: string;
      category: string;
      tree: unknown[];
    };
    expect(firstBody.key).toMatch(/^consts--/);
    expect(firstBody.category).toBe('常量');
    expect(Array.isArray(firstBody.tree)).toBe(true);
    // 成功后重置：回到 0 个组件
    expect(await screen.findByRole('button', { name: /全部保存（0 个组件）/ })).toBeInTheDocument();
  });

  it('保存失败展示错误 Alert', async () => {
    requestMock.mockRejectedValue(new Error('quota exceeded'));
    renderModal();
    uploadFile(jsonFile('{"阵营":["联盟"]}'));

    fireEvent.click(await screen.findByRole('button', { name: /全部保存（1 个组件）/ }));

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('quota exceeded'));
  });
});

describe('取消', () => {
  it('底部取消按钮重置并回调 onCancel', async () => {
    const { onCancel } = renderModal();
    uploadFile(jsonFile('{"阵营":["联盟"]}'));
    await waitFor(() => expect(screen.getByText(/常量预览（1 个/)).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));
    expect(onCancel).toHaveBeenCalled();
  });
});

// ==================== 覆盖率缺口补充（v8 branch/function）====================

describe('Excel 解析失败（beforeUpload catch）', () => {
  it('损坏的 xlsx 字节流 → Alert 展示 XLSX 抛出的错误信息', async () => {
    renderModal();
    // 垃圾文本会被 sheetjs 降级为 PRN 明文解析而不抛错；
    // PNG 魔数走 firstbyte 分发，确定性抛 Error('PNG Image File is not a spreadsheet')
    const pngBytes = new File(
      [new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])],
      'broken.xlsx',
      { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' },
    );
    uploadFile(pngBytes);

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('PNG Image File is not a spreadsheet'),
    );
    expect(screen.queryByText(/常量预览/)).not.toBeInTheDocument();
  });
});

describe('JSON 解析抛非 Error 值（防御兜底）', () => {
  it('jsonToFields 抛出原始字符串 → 提示「JSON 解析失败」', async () => {
    // 单次抛非 Error 值：命中 e instanceof Error 的 false 分支（intl 兜底文案）
    (jsonToFields as unknown as jest.Mock).mockImplementationOnce(() => {
      throw '原始字符串异常';
    });
    renderModal();
    uploadFile(jsonFile('{"阵营":["联盟"]}'));

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('JSON 解析失败'));
  });
});

describe('空名常量兜底（f.title || f.key）', () => {
  it('空字符串常量名：保存的模板 name 与 tree 标题回退空值', async () => {
    const onSaved = jest.fn();
    renderModal(onSaved);
    // 对象 key 为空串：jsonToFields 仅按 options 过滤，产出 title/key 俱空的字段
    uploadFile(jsonFile('{"": ["联盟"]}'));

    fireEvent.click(await screen.findByRole('button', { name: /全部保存（1 个组件）/ }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('1 个常量组件'));

    const body = requestMock.mock.calls[0][1].data as {
      key: string;
      name: { 'zh-CN': string; 'en-US': string };
      tree: Array<{ props: { title?: string } }>;
    };
    expect(body.key).toMatch(/^consts--/);
    // f.title || f.key 双分支（zh-CN/en-US）与 staticFormNodeFromFields 标题兜底
    expect(body.name).toEqual({ 'zh-CN': '', 'en-US': '' });
    expect(body.tree[0]?.props.title).toBe('');
  });
});

describe('保存失败（reject 非 Error 值）', () => {
  it('request reject 字符串 → 提示「保存失败」兜底', async () => {
    requestMock.mockRejectedValue('配额超限');
    renderModal();
    uploadFile(jsonFile('{"阵营":["联盟"]}'));

    fireEvent.click(await screen.findByRole('button', { name: /全部保存（1 个组件）/ }));

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('保存失败'));
  });
});

describe('右上角 X 关闭（Modal onCancel）', () => {
  it('X 按钮：重置字段并回调 onCancel', async () => {
    const { onCancel } = renderModal();
    uploadFile(jsonFile('{"阵营":["联盟"]}'));
    await waitFor(() => expect(screen.getByText(/常量预览（1 个/)).toBeInTheDocument());

    const closeIcon = document.querySelector('.ant-modal-close') as HTMLElement | null;
    expect(closeIcon).not.toBeNull();
    fireEvent.click(closeIcon as HTMLElement);
    expect(onCancel).toHaveBeenCalledTimes(1);
    // reset 生效：保存计数归零
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /全部保存（0 个组件）/ })).toBeInTheDocument(),
    );
  });
});

describe('常量预览编辑器回写（ConstantFieldsEditor onChange）', () => {
  it('编辑器回写单字段 schema → fields 经 schemaToFields 收敛（计数联动）', async () => {
    renderModal();
    uploadFile(jsonFile('[{"name":"甲","options":["a"]},{"name":"乙","options":["b"]}]'));
    expect(await screen.findByRole('button', { name: /全部保存（2 个组件）/ })).toBeInTheDocument();

    // mock 编辑器的回写按钮 → onChange(schema) → setFields(schemaToFields(schema))
    fireEvent.click(screen.getByTestId('cfe-change'));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /全部保存（1 个组件）/ })).toBeInTheDocument(),
    );
  });
});
