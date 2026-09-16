/**
 * ConstantImportModal（导入常量向导）：
 * JSON 导入（对象映射/数组形态/空结果提示/解析失败 Alert）、
 * Excel 导入（XLSX 真实字节流，长表聚合）、
 * 保存（空字段提示、逐常量 POST component-templates、成功回调与重置、失败 Alert）、
 * 取消（重置 + onCancel）。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor, configure } from '@testing-library/react';
import * as XLSX from 'xlsx';
import { request } from '@umijs/max';
import ConstantImportModal from '../ConstantImportModal';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

// @umijs/max 用 setupTests 的全局 mock（request 即 jest.fn）
const requestMock = request as unknown as jest.Mock;

jest.mock('../ConstantFieldsEditor', () => ({
  __esModule: true,
  default: () => <div data-testid="constant-fields-editor" />,
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
